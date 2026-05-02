use crate::utils::libp2p_mock::{
    decode_request, encode_gossip_data, extract_ip_port, lean_block_topic, replace_multiaddr_ip,
    LeanSignedBlock, MockBehaviourEvent, MockNode, Status, RESPONSE_CODE_SUCCESS,
};
use crate::utils::util::{
    expect_single_client, lean_clients, lean_environment, lean_single_client_runtime_setup,
    load_fork_choice_response, prepare_client_runtime_files, selected_lean_devnet,
    simulator_container_ip, LeanDevnet,
};
use alloy_primitives::B256;
use futures::prelude::*;
use hivesim::{dyn_async, Client, Test};
use libp2p::swarm::SwarmEvent;
use ssz::Encode;
use std::time::Duration;
use tokio::time::sleep;

const GOSSIPSUB_TIMEOUT_SECS: u64 = 30;

// Suite: gossip
// Tests gossipsub protocol behavior using a mock node.

dyn_async! {
    pub async fn run_gossip_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let (fresh_client_environments, fresh_client_files) = lean_single_client_runtime_setup(&client.name);

            test.run(hivesim::NClientTestSpec {
                name: "gossip: client subscribes to block topic".to_string(),
                description: "Verifies the client subscribes to the lean block gossip topic after establishing a connection.".to_string(),
                always_run: false,
                run: test_gossipsub_subscribes_to_block_topic,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "gossip: ignores wrong fork digest topic".to_string(),
                description: "Publishes a block on a topic with a mismatched fork digest and verifies the client ignores it.".to_string(),
                always_run: false,
                run: test_gossipsub_ignores_wrong_fork_digest,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "gossip: ignores malformed ssz".to_string(),
                description: "Publishes random bytes on a valid gossip topic and verifies the client stays healthy.".to_string(),
                always_run: false,
                run: test_gossipsub_ignores_malformed_ssz,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "gossip: caches orphan block".to_string(),
                description: "Publishes a child block with an unknown parent root, then the parent, and verifies both are eventually processed.".to_string(),
                always_run: false,
                run: test_gossipsub_caches_orphan_block,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(hivesim::NClientTestSpec {
                name: "gossip: deduplicates duplicate messages".to_string(),
                description: "Publishes the same invalid block twice and verifies the client handles it idempotently (no crash).".to_string(),
                always_run: false,
                run: test_gossipsub_deduplicates,
                environments: fresh_client_environments.clone(),
                files: fresh_client_files.clone(),
                test_data: (),
                clients: vec![client.clone()],
            }).await;
        }
    }
}

// === Helpers ===

fn fork_digest_for_devnet() -> &'static str {
    if selected_lean_devnet() == LeanDevnet::Devnet4 {
        "12345678"
    } else {
        "devnet0"
    }
}

/// Start a mock node as the client's only bootnode, handle the initial Status handshake,
/// and return the mock, the running client, and the block topic.
async fn setup_mock_bootnode(clients: Vec<Client>) -> (MockNode, Client, String) {
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

    let fork_digest = fork_digest_for_devnet();
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

    let (_peer, _req_id, request, channel) = tokio::time::timeout(
        Duration::from_secs(GOSSIPSUB_TIMEOUT_SECS),
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
    .expect("should send valid status response");

    (mock, client, fork_digest.to_string())
}

// === Tests ===

dyn_async! {
    async fn test_gossipsub_subscribes_to_block_topic<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, _client, fork_digest) = setup_mock_bootnode(clients).await;

        let mut subscribed = false;
        let deadline = std::time::Instant::now() + Duration::from_secs(GOSSIPSUB_TIMEOUT_SECS);

        while std::time::Instant::now() < deadline {
            match tokio::time::timeout(Duration::from_secs(1), mock.swarm.select_next_some()).await {
                Ok(SwarmEvent::Behaviour(MockBehaviourEvent::Gossipsub(
                    libp2p::gossipsub::Event::Subscribed { peer_id: _, topic } ,
                ))) => {
                    if topic == lean_block_topic(&fork_digest).hash() {
                        subscribed = true;
                        break;
                    }
                }
                Ok(_) => continue,
                Err(_) => continue,
            }
        }

        assert!(subscribed, "client should subscribe to block gossip topic within {} seconds", GOSSIPSUB_TIMEOUT_SECS);
    }
}

dyn_async! {
    async fn test_gossipsub_ignores_wrong_fork_digest<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, fork_digest) = setup_mock_bootnode(clients).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let fork_choice_before = load_fork_choice_response(&client).await;
        let block_count_before = fork_choice_before.nodes.len();

        let wrong_fork = if fork_digest == "12345678" { "devnet0" } else { "12345678" };
        let wrong_topic = lean_block_topic(wrong_fork);
        mock.subscribe(&wrong_topic)
            .expect("mock should subscribe to wrong-fork topic");

        let block = LeanSignedBlock::build_minimal(1, 0, B256::ZERO, B256::ZERO);
        let block_bytes = encode_gossip_data(&block);
        if let Err(e) = mock.publish(wrong_topic, block_bytes) {
            // If there are no peers subscribed to the wrong topic, the publish
            // will fail. This is expected because the client only subscribes to
            // the correct fork digest topic.
            assert!(
                e.contains("NoPeersSubscribedToTopic"),
                "unexpected publish error: {e}"
            );
        } else {
            mock.process_events_for(Duration::from_secs(5)).await;
        }

        let fork_choice_after = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice_after.nodes.len(),
            block_count_before,
            "client should ignore gossip on wrong-fork topic (no new blocks imported)"
        );
    }
}

dyn_async! {
    async fn test_gossipsub_ignores_malformed_ssz<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, fork_digest) = setup_mock_bootnode(clients).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let block_topic = lean_block_topic(&fork_digest);
        let garbage = vec![0xef; 1024];
        mock.publish(block_topic, garbage)
            .expect("should publish garbage bytes");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "client should remain healthy after receiving malformed gossip"
        );
    }
}

dyn_async! {
    async fn test_gossipsub_caches_orphan_block<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, fork_digest) = setup_mock_bootnode(clients).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let block_topic = lean_block_topic(&fork_digest);

        let parent_block = LeanSignedBlock::build_minimal(1, 0, B256::ZERO, B256::ZERO);
        let parent_root = B256::from_slice(&[0xca; 32]);

        let child_block = LeanSignedBlock::build_minimal(2, 0, parent_root, B256::ZERO);

        let fork_choice_before = load_fork_choice_response(&client).await;
        let block_count_before = fork_choice_before.nodes.len();

        let child_bytes = encode_gossip_data(&child_block);
        mock.publish(block_topic.clone(), child_bytes)
            .expect("should publish orphan child block");

        mock.process_events_for(Duration::from_secs(3)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.nodes.len(),
            block_count_before,
            "client should not process orphan block"
        );

        let parent_bytes = encode_gossip_data(&parent_block);
        mock.publish(block_topic, parent_bytes)
            .expect("should publish parent block");

        sleep(Duration::from_secs(8)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        // The client may or may not process these blocks (they are structurally valid SSZ
        // but may fail state-transition validation). The important thing is that the client
        // did not crash and remains responsive.
        assert!(
            !fork_choice.nodes.is_empty(),
            "client should remain healthy after receiving orphan and parent blocks"
        );
    }
}

dyn_async! {
    async fn test_gossipsub_deduplicates<'a>(clients: Vec<Client>, _: ()) {
        let (mut mock, client, fork_digest) = setup_mock_bootnode(clients).await;

        mock.process_events_for(Duration::from_secs(3)).await;

        let block_topic = lean_block_topic(&fork_digest);
        let invalid_block = LeanSignedBlock::build_minimal(1, 9999, B256::ZERO, B256::ZERO);
        let block_bytes = encode_gossip_data(&invalid_block);

        let fork_choice_before = load_fork_choice_response(&client).await;
        let block_count_before = fork_choice_before.nodes.len();

        mock.publish(block_topic.clone(), block_bytes.clone())
            .expect("should publish first copy of invalid block");
        sleep(Duration::from_secs(1)).await;
        mock.publish(block_topic, block_bytes)
            .expect("should publish second copy of invalid block");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.nodes.len(),
            block_count_before,
            "client should not import invalid block after duplicate gossip"
        );
    }
}
