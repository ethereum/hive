use crate::utils::libp2p_mock::{
    decode_gossip_block, decode_request, encode_gossip_block, extract_ip_port, lean_block_topic,
    replace_multiaddr_ip, LeanBlock, MockNode, Status, RESPONSE_CODE_SUCCESS,
};
use crate::utils::util::{
    current_unix_time, expect_single_client, lean_clients, lean_environment,
    lean_single_client_runtime_setup, load_fork_choice_response, prepare_client_runtime_files,
    selected_lean_devnet, simulator_container_ip, ForkChoiceResponse,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use libp2p::gossipsub::IdentTopic;
use ssz::Encode;
use std::time::{Duration, Instant};

const REQRESP_LIBP2P_TIMEOUT_SECS: u64 = 30;
const VALID_BLOCK_TIMEOUT_SECS: u64 = 75;
const VALIDATION_GENESIS_DELAY_SECS: u64 = 30;
const FUTURE_HORIZON_GENESIS_DELAY_SECS: u64 = 60;
const HIVE_LEAN_GENESIS_TIME: &str = "HIVE_LEAN_GENESIS_TIME";

struct CapturedValidBlock {
    block: LeanBlock,
    gossip_bytes: Vec<u8>,
}

// Suite: validation
// Tests that clients properly validate blocks,
// rejecting invalid data according to the lean consensus spec.

async fn setup_mock_for_validation(
    clients: Vec<Client>,
    genesis_delay_secs: u64,
) -> (MockNode, Client, IdentTopic) {
    let client = expect_single_client(clients);
    let client_type = client.kind.clone();
    let test = client.test.clone();

    let mut mock = MockNode::new().expect("failed to create mock node");
    let listen_addr = mock
        .get_listen_address()
        .await
        .expect("mock node should bind to an address");

    let _mock_peer_id = mock.local_peer_id();
    let external_addr = replace_multiaddr_ip(&listen_addr, simulator_container_ip());
    let (ip, port) =
        extract_ip_port(&external_addr).expect("mock listen address should have IP and port");
    let mock_enr = mock
        .enr_string(
            match ip {
                std::net::IpAddr::V4(v4) => v4,
                _ => panic!("expected IPv4"),
            },
            port,
        )
        .expect("should generate ENR for mock node");

    let fork_digest = if selected_lean_devnet().uses_latest_leanspec_format() {
        "12345678"
    } else {
        "devnet0"
    };
    let block_topic = lean_block_topic(fork_digest);
    mock.subscribe(&block_topic)
        .expect("mock node should subscribe to block topic");

    let mut environment = lean_environment();
    environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
    environment.insert(
        HIVE_LEAN_GENESIS_TIME.to_string(),
        (current_unix_time() + genesis_delay_secs).to_string(),
    );

    let files = prepare_client_runtime_files(&client_type, &environment)
        .unwrap_or_else(|e| panic!("failed to prepare client files: {e}"));

    let client = test
        .start_client_with_files(client_type, Some(environment), Some(files))
        .await;

    let (_peer, _req_id, request, channel) = tokio::time::timeout(
        Duration::from_secs(REQRESP_LIBP2P_TIMEOUT_SECS),
        mock.wait_for_request(),
    )
    .await
    .expect("client should connect and send a request")
    .expect("mock should receive a request");

    let decompressed = decode_request(&request).expect("should be able to decode request");
    let client_status = Status::from_ssz_bytes(&decompressed)
        .expect("first request should be a valid Status message");

    mock.send_response(
        channel,
        vec![(RESPONSE_CODE_SUCCESS, client_status.as_ssz_bytes())],
    )
    .expect("should send valid test response");

    (mock, client, block_topic)
}

fn block_is_imported(fork_choice: &ForkChoiceResponse, block: &LeanBlock) -> bool {
    let block_root = block.block_root();
    fork_choice.nodes.iter().any(|node| node.root == block_root)
}

async fn wait_for_client_generated_valid_block(
    mock: &mut MockNode,
    client: &Client,
    block_topic: &IdentTopic,
) -> CapturedValidBlock {
    let deadline = Instant::now() + Duration::from_secs(VALID_BLOCK_TIMEOUT_SECS);
    let expected_topic = block_topic.hash();
    let mut last_decoding_error = None;

    while Instant::now() < deadline {
        mock.process_events_for(Duration::from_secs(1)).await;

        for (_peer, topic, gossip_bytes) in mock.take_gossip_messages() {
            if topic != expected_topic {
                continue;
            }

            let block = match decode_gossip_block(&gossip_bytes) {
                Ok(block) => block,
                Err(err) => {
                    last_decoding_error = Some(err.to_string());
                    continue;
                }
            };

            let fork_choice = load_fork_choice_response(client).await;
            if block_is_imported(&fork_choice, &block) {
                return CapturedValidBlock {
                    block,
                    gossip_bytes,
                };
            }
        }
    }

    panic!(
        "client did not gossip and import a valid block within {} seconds; last decoding error: {}",
        VALID_BLOCK_TIMEOUT_SECS,
        last_decoding_error.as_deref().unwrap_or("none")
    );
}

async fn wait_for_post_genesis_import(mock: &mut MockNode, client: &Client) -> ForkChoiceResponse {
    let deadline = Instant::now() + Duration::from_secs(VALID_BLOCK_TIMEOUT_SECS);

    while Instant::now() < deadline {
        mock.process_events_for(Duration::from_secs(1)).await;
        let fork_choice = load_fork_choice_response(client).await;
        if fork_choice.nodes.iter().any(|node| node.slot > 0) {
            return fork_choice;
        }
    }

    panic!(
        "client did not import any post-genesis block within {} seconds after invalid gossip",
        VALID_BLOCK_TIMEOUT_SECS
    );
}

fn assert_block_absent(fork_choice: &ForkChoiceResponse, block: &LeanBlock, context: &str) {
    assert!(
        !block_is_imported(fork_choice, block),
        "{context}: block {} must not appear in fork choice",
        block.block_root()
    );
}

dyn_async! {
    pub async fn run_validation_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let (fresh_client_environments, fresh_client_files) = lean_single_client_runtime_setup(&client.name);

            test.run(hivesim::NClientTestSpec {
                name: "validation: rejects block with invalid proposer".to_string(),
                description: "Checks that the client rejects blocks where the proposer index does not match the expected proposer for that slot.".to_string(),
                always_run: false,
                run: test_rejects_invalid_proposer,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "validation: rejects block with invalid parent root".to_string(),
                description: "Checks that the client rejects blocks where the parent root does not match the latest block header.".to_string(),
                always_run: false,
                run: test_rejects_invalid_parent_root,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "validation: rejects block with invalid state root".to_string(),
                description: "Checks that the client rejects blocks where the state root does not match the computed post-state.".to_string(),
                always_run: false,
                run: test_rejects_invalid_state_root,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "validation: rejects block beyond future-slot horizon".to_string(),
                description: "Publishes a known-parent block two slots beyond the client's clock and verifies that it is not imported; LeanSpec fixtures isolate the exact horizon rejection reason.".to_string(),
                always_run: false,
                run: test_rejects_block_beyond_future_slot_horizon,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "validation: accepts valid block after invalid gossip".to_string(),
                description: "Publishes an invalid block, verifies it is not imported, then verifies that the same client still produces and imports a valid block.".to_string(),
                always_run: false,
                run: test_accepts_valid_block_after_invalid_gossip,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "validation: duplicate valid block is idempotent".to_string(),
                description: "Captures a client-generated signed block, attempts an exact gossip replay, and verifies that network deduplication or client processing leaves one fork-choice entry.".to_string(),
                always_run: false,
                run: test_duplicate_valid_block_is_idempotent,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;
        }
    }
}

dyn_async! {
    async fn test_rejects_invalid_proposer<'a>(clients: Vec<Client>, _: ()) {

        let (mut mock, client, block_topic) =
    setup_mock_for_validation(clients, VALIDATION_GENESIS_DELAY_SECS).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanBlock::build_minimal(
            1, 9999, B256::ZERO, B256::ZERO
        );
        let block_bytes = encode_gossip_block(&invalid_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish invalid block");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.nodes.len(), 1,
            "client should still only have genesis after rejecting invalid proposer"
        );
        assert_eq!(
            fork_choice.nodes[0].slot, 0,
            "client should remain at genesis after rejecting invalid proposer"
        );
    }
}

dyn_async! {
    async fn test_rejects_block_beyond_future_slot_horizon<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, block_topic) =
            setup_mock_for_validation(clients, FUTURE_HORIZON_GENESIS_DELAY_SECS).await;

        mock.process_events_for(Duration::from_secs(2)).await;

        let fork_choice_before = load_fork_choice_response(&client).await;
        let genesis = fork_choice_before
            .nodes
            .iter()
            .find(|node| node.slot == 0)
            .expect("fresh validation client should expose its genesis block");
        let future_block = LeanBlock::build_minimal(2, 0, genesis.root, B256::ZERO);

        mock.publish(block_topic, encode_gossip_block(&future_block))
            .expect("should publish block beyond future-slot horizon");
        mock.process_events_for(Duration::from_secs(3)).await;

        let fork_choice_after = load_fork_choice_response(&client).await;
        assert_block_absent(
            &fork_choice_after,
            &future_block,
            "block two slots beyond the pre-genesis clock",
        );
        assert_eq!(
            fork_choice_after.nodes.len(),
            fork_choice_before.nodes.len(),
            "future block must not mutate the fork-choice store",
        );
        assert_eq!(
            fork_choice_after.head, fork_choice_before.head,
            "future block must not move the fork-choice head",
        );
    }
}

dyn_async! {
    async fn test_accepts_valid_block_after_invalid_gossip<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, block_topic) =
            setup_mock_for_validation(clients, VALIDATION_GENESIS_DELAY_SECS).await;

        mock.process_events_for(Duration::from_secs(2)).await;

        let fork_choice_before = load_fork_choice_response(&client).await;
        let genesis = fork_choice_before
            .nodes
            .iter()
            .find(|node| node.slot == 0)
            .expect("fresh validation client should expose its genesis block");
        let invalid_block = LeanBlock::build_minimal(1, u64::MAX, genesis.root, B256::ZERO);

        mock.publish(block_topic.clone(), encode_gossip_block(&invalid_block))
            .expect("should publish invalid block before recovery check");
        mock.process_events_for(Duration::from_secs(3)).await;

        let fork_choice_after_invalid = load_fork_choice_response(&client).await;
        assert_block_absent(
            &fork_choice_after_invalid,
            &invalid_block,
            "invalid predecessor",
        );
        assert_eq!(
            fork_choice_after_invalid.head, fork_choice_before.head,
            "invalid gossip must not move the fork-choice head",
        );

        // The client may legitimately disconnect or penalize the peer that sent the
        // invalid block, so recovery is observed through its own fork-choice store.
        let fork_choice_after_valid = wait_for_post_genesis_import(&mut mock, &client).await;
        assert!(
            fork_choice_after_valid
                .nodes
                .iter()
                .any(|node| node.slot > 0),
            "client should continue beyond genesis after invalid gossip",
        );
    }
}

dyn_async! {
    async fn test_duplicate_valid_block_is_idempotent<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, block_topic) =
            setup_mock_for_validation(clients, VALIDATION_GENESIS_DELAY_SECS).await;
        let valid = wait_for_client_generated_valid_block(&mut mock, &client, &block_topic).await;
        let block_root = valid.block.block_root();

        let fork_choice_before = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice_before
                .nodes
                .iter()
                .filter(|node| node.root == block_root)
                .count(),
            1,
            "captured valid block should have exactly one fork-choice entry before replay",
        );

        match mock.publish(block_topic, valid.gossip_bytes) {
            Ok(()) => mock.process_events_for(Duration::from_secs(2)).await,
            Err(err) if err.contains("Duplicate") => {
                // Exact payloads have the same gossipsub message ID. Suppression at
                // this layer is the preferred idempotent outcome; if delivery occurs,
                // the fork-choice assertion below covers the client-processing path.
            }
            Err(err) => panic!("failed to attempt signed-block replay: {err}"),
        }

        let fork_choice_after = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice_after
                .nodes
                .iter()
                .filter(|node| node.root == block_root)
                .count(),
            1,
            "duplicate delivery must not create a second fork-choice entry",
        );
        assert!(
            fork_choice_after.nodes.iter().any(|node| node.root == block_root),
            "duplicate delivery must not remove or corrupt the original valid block",
        );
    }
}

dyn_async! {
    async fn test_rejects_invalid_parent_root<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, block_topic) =
    setup_mock_for_validation(clients, VALIDATION_GENESIS_DELAY_SECS).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanBlock::build_minimal(
            1, 0, B256::from_slice(&[0xde; 32]), B256::ZERO
        );
        let block_bytes = encode_gossip_block(&invalid_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish invalid block");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.nodes.len(), 1,
            "client should still only have genesis after rejecting invalid parent root"
        );
        assert_eq!(
            fork_choice.nodes[0].slot, 0,
            "client should remain at genesis after rejecting invalid parent root"
        );
    }
}

dyn_async! {
    async fn test_rejects_invalid_state_root<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, block_topic) =
    setup_mock_for_validation(clients, VALIDATION_GENESIS_DELAY_SECS).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanBlock::build_minimal(
            1, 0, B256::ZERO, B256::from_slice(&[0xbe; 32])
        );
        let block_bytes = encode_gossip_block(&invalid_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish invalid block");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.nodes.len(), 1,
            "client should still only have genesis after rejecting invalid state root"
        );
        assert_eq!(
            fork_choice.nodes[0].slot, 0,
            "client should remain at genesis after rejecting invalid state root"
        );
    }
}
