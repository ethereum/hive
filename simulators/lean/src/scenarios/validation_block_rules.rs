use crate::utils::libp2p_mock::{
    decode_request, encode_gossip_data, extract_ip_port, lean_block_topic, replace_multiaddr_ip,
    LeanSignedBlock, MockNode, Status, RESPONSE_CODE_SUCCESS,
};
use crate::utils::util::{
    expect_single_client, lean_clients, lean_environment, lean_single_client_runtime_setup,
    load_fork_choice_response, prepare_client_runtime_files, selected_lean_devnet,
    simulator_container_ip,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use libp2p::gossipsub::IdentTopic;
use ssz::Encode;
use std::time::Duration;

const REQRESP_LIBP2P_TIMEOUT_SECS: u64 = 30;
const GOSSIP_SETTLE_SECS: u64 = 5;
const SUBSCRIPTION_WAIT_SECS: u64 = 30;
const FAR_FUTURE_SLOT: u64 = 1_000_000_000;

// Extends the `validation` suite with a block gossip-validation rule not covered
// there: a future-slot block must not be imported. The test publishes one crafted
// block and asserts fork choice stays at genesis.

dyn_async! {
    pub async fn run_validation_block_rules_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let (fresh_client_environments, fresh_client_files) =
                lean_single_client_runtime_setup(&client.name);

            test.run(hivesim::NClientTestSpec {
                name: "validation: rejects block from a future slot".to_string(),
                description: "Checks that the client refuses to import a gossiped block whose slot is far beyond the current wall-clock slot, leaving fork choice at genesis.".to_string(),
                always_run: false,
                run: test_rejects_future_slot_block,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;
        }
    }
}

/// Start the client under test behind a single mock peer, complete the Status
/// handshake, wait until the client has subscribed to the block topic, and
/// return the running client, the mock, and the block topic.
async fn launch_client_behind_mock_peer(client: Client) -> (MockNode, Client, IdentTopic) {
    let client_type = client.kind.clone();
    let client_name = client_type.clone();
    let test = client.test.clone();

    let mut mock = MockNode::new().expect("failed to create mock node");
    let listen_addr = mock
        .get_listen_address()
        .await
        .expect("mock node should bind to an address");
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
        .expect("mock should subscribe to block topic");

    let mut environment = lean_environment();
    environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
    let files = prepare_client_runtime_files(&client_type, &environment)
        .unwrap_or_else(|e| panic!("failed to prepare client files: {e}"));
    let client = test
        .start_client_with_files(client_type, Some(environment), Some(files))
        .await;

    let (peer, _req_id, request, channel) = tokio::time::timeout(
        Duration::from_secs(REQRESP_LIBP2P_TIMEOUT_SECS),
        mock.wait_for_request(),
    )
    .await
    .expect("client should connect and send a request")
    .expect("mock should receive a request");

    let decompressed = decode_request(&request).expect("should be able to decode request");
    let client_status = Status::from_ssz_bytes(&decompressed)
        .expect("first request should be a valid Status message");

    // Echo the client's Status so it treats the mock as a same-chain peer.
    mock.send_response(
        channel,
        vec![(RESPONSE_CODE_SUCCESS, client_status.as_ssz_bytes())],
    )
    .expect("should send valid status response");

    mock.process_events_for(Duration::from_secs(3)).await;

    // Wait for the client to join the block topic so the publish below can't
    // race the SUBSCRIBE (which would let the assertion pass vacuously).
    let subscribed = mock
        .wait_for_subscription(
            &block_topic,
            &peer,
            Duration::from_secs(SUBSCRIPTION_WAIT_SECS),
        )
        .await;
    assert!(
        subscribed,
        "client {client_name} never subscribed to the block gossip topic within {SUBSCRIPTION_WAIT_SECS}s (possible fork-digest/topic mismatch)"
    );

    (mock, client, block_topic)
}

/// Assert the client still holds only the genesis block, i.e. the crafted block
/// was not imported.
async fn assert_remains_at_genesis(client: &Client, rule: &str) {
    let fork_choice = load_fork_choice_response(client).await;
    assert_eq!(
        fork_choice.nodes.len(),
        1,
        "client should still only have genesis after rejecting {rule}"
    );
    assert_eq!(
        fork_choice.nodes[0].slot, 0,
        "client should remain at genesis after rejecting {rule}"
    );
    assert_eq!(
        fork_choice.head, fork_choice.nodes[0].root,
        "fork-choice head should still point at the genesis node after rejecting {rule}"
    );
}

/// Genesis block root of a fresh client, used as a valid parent so the slot is
/// the only disqualifying property of the crafted block.
async fn genesis_block_root(client: &Client) -> B256 {
    let fork_choice = load_fork_choice_response(client).await;
    assert_eq!(
        fork_choice.nodes.len(),
        1,
        "fresh client should expose exactly the genesis node before any block is published"
    );
    fork_choice.nodes[0].root
}

dyn_async! {
    async fn test_rejects_future_slot_block<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let (mut mock, client, block_topic) = launch_client_behind_mock_peer(client).await;

        let genesis_root = genesis_block_root(&client).await;
        let future_block =
            LeanSignedBlock::build_minimal(FAR_FUTURE_SLOT, 0, genesis_root, B256::ZERO);
        let block_bytes = encode_gossip_data(&future_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish future-slot block");

        mock.process_events_for(Duration::from_secs(GOSSIP_SETTLE_SECS)).await;

        assert_remains_at_genesis(&client, "a future-slot block").await;
    }
}
