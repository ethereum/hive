use crate::utils::libp2p_mock::{
    decode_request, encode_gossip_data, extract_ip_port, lean_block_topic, replace_multiaddr_ip,
    LeanSignedBlock, MockNode, Status, RESPONSE_CODE_SUCCESS,
};
use crate::utils::util::{
    expect_single_client, lean_clients, lean_environment, lean_single_client_runtime_setup,
    load_fork_choice_response, prepare_client_runtime_files, selected_lean_devnet,
    simulator_container_ip, LeanDevnet,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use ssz::Encode;
use std::time::Duration;
use tokio::time::sleep;

const REQRESP_LIBP2P_TIMEOUT_SECS: u64 = 30;

// Suite: validation
// Tests that clients properly validate blocks,
// rejecting invalid data according to the lean consensus spec.

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
        }
    }
}

dyn_async! {
    async fn test_rejects_invalid_proposer<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let client_type = client.kind.clone();
        let test = client.test.clone();

        let mut mock = MockNode::new().expect("failed to create mock node");
        let listen_addr = mock.get_listen_address().await
            .expect("mock node should bind to an address");
        let _mock_peer_id = mock.local_peer_id();
        let external_addr = replace_multiaddr_ip(&listen_addr, simulator_container_ip());
        let (ip, port) = extract_ip_port(&external_addr)
            .expect("mock listen address should have IP and port");
        let mock_enr = mock.enr_string(
            match ip {
                std::net::IpAddr::V4(v4) => v4,
                _ => panic!("expected IPv4"),
            },
            port,
        ).expect("should generate ENR for mock node");

        let fork_digest = if selected_lean_devnet() == LeanDevnet::Devnet4 {
            "12345678"
        } else {
            "devnet0"
        };
        let block_topic = lean_block_topic(fork_digest);
        mock.subscribe(&block_topic).expect("mock should subscribe to block topic");

        let mut environment = lean_environment();
        environment.insert("HIVE_BOOTNODES".to_string(), mock_enr.clone());
        let files = prepare_client_runtime_files(
            &client_type, &environment)
            .unwrap_or_else(|e| panic!("failed to prepare client files: {e}"));
        let client = test.start_client_with_files(client_type, Some(environment), Some(files)).await;


        let (_peer, _req_id, request, channel) = tokio::time::timeout(
            Duration::from_secs(REQRESP_LIBP2P_TIMEOUT_SECS),
            mock.wait_for_request()
        ).await
            .expect("client should connect and send a request")
            .expect("mock should receive a request");

        let decompressed = decode_request(&request)
            .expect("should be able to decode request");
        let client_status = Status::from_ssz_bytes(&decompressed)
            .expect("first request should be a valid Status message");

        // Echo the client's Status so it thinks we're on the same chain.
        mock.send_response(channel, vec![
            (RESPONSE_CODE_SUCCESS, client_status.as_ssz_bytes())
        ]).expect("should send valid status response");

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanSignedBlock::build_minimal(
            1, 9999, B256::ZERO, B256::ZERO
        );
        let block_bytes = encode_gossip_data(&invalid_block);
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
    async fn test_rejects_invalid_parent_root<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let client_type = client.kind.clone();
        let test = client.test.clone();

        let mut mock = MockNode::new().expect("failed to create mock node");
        let listen_addr = mock.get_listen_address().await
            .expect("mock node should bind to an address");
        let _mock_peer_id = mock.local_peer_id();
        let external_addr = replace_multiaddr_ip(&listen_addr, simulator_container_ip());
        let (ip, port) = extract_ip_port(&external_addr)
            .expect("mock listen address should have IP and port");
        let mock_enr = mock.enr_string(
            match ip {
                std::net::IpAddr::V4(v4) => v4,
                _ => panic!("expected IPv4"),
            },
            port,
        ).expect("should generate ENR for mock node");

        let fork_digest = if selected_lean_devnet() == LeanDevnet::Devnet4 {
            "12345678"
        } else {
            "devnet0"
        };
        let block_topic = lean_block_topic(fork_digest);
        mock.subscribe(&block_topic).expect("mock should subscribe to block topic");

        let mut environment = lean_environment();
        environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
        let files = prepare_client_runtime_files(
            &client_type, &environment)
            .unwrap_or_else(|e| panic!("failed to prepare client files: {e}"));
        let client = test.start_client_with_files(client_type, Some(environment), Some(files)).await;

        let (_peer, _req_id, request, channel) = tokio::time::timeout(
            Duration::from_secs(REQRESP_LIBP2P_TIMEOUT_SECS),
            mock.wait_for_request()
        ).await
            .expect("client should connect and send a request")
            .expect("mock should receive a request");

        let decompressed = decode_request(&request)
            .expect("should be able to decode request");
        let client_status = Status::from_ssz_bytes(&decompressed)
            .expect("first request should be a valid Status message");

        // Echo the client's Status so it thinks we're on the same chain.
        mock.send_response(channel, vec![
            (RESPONSE_CODE_SUCCESS, client_status.as_ssz_bytes())
        ]).expect("should send valid status response");

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanSignedBlock::build_minimal(
            1, 0, B256::from_slice(&[0xde; 32]), B256::ZERO
        );
        let block_bytes = encode_gossip_data(&invalid_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish invalid block");

        sleep(Duration::from_secs(5)).await;

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
        let client = expect_single_client(clients);
        let client_type = client.kind.clone();
        let test = client.test.clone();

        let mut mock = MockNode::new().expect("failed to create mock node");
        let listen_addr = mock.get_listen_address().await
            .expect("mock node should bind to an address");
        let _mock_peer_id = mock.local_peer_id();
        let external_addr = replace_multiaddr_ip(&listen_addr, simulator_container_ip());
        let (ip, port) = extract_ip_port(&external_addr)
            .expect("mock listen address should have IP and port");
        let mock_enr = mock.enr_string(
            match ip {
                std::net::IpAddr::V4(v4) => v4,
                _ => panic!("expected IPv4"),
            },
            port,
        ).expect("should generate ENR for mock node");

        let fork_digest = if selected_lean_devnet() == LeanDevnet::Devnet4 {
            "12345678"
        } else {
            "devnet0"
        };
        let block_topic = lean_block_topic(fork_digest);
        mock.subscribe(&block_topic).expect("mock should subscribe to block topic");

        let mut environment = lean_environment();
        environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
        let files = prepare_client_runtime_files(
            &client_type, &environment)
            .unwrap_or_else(|e| panic!("failed to prepare client files: {e}"));
        let client = test.start_client_with_files(client_type, Some(environment), Some(files)).await;

        let (_peer, _req_id, request, channel) = tokio::time::timeout(
            Duration::from_secs(REQRESP_LIBP2P_TIMEOUT_SECS),
            mock.wait_for_request()
        ).await
            .expect("client should connect and send a request")
            .expect("mock should receive a request");

        let decompressed = decode_request(&request)
            .expect("should be able to decode request");
        let client_status = Status::from_ssz_bytes(&decompressed)
            .expect("first request should be a valid Status message");

        // Echo the client's Status so it thinks we're on the same chain.
        mock.send_response(channel, vec![
            (RESPONSE_CODE_SUCCESS, client_status.as_ssz_bytes())
        ]).expect("should send valid status response");

        mock.process_events_for(Duration::from_secs(3)).await;

        let invalid_block = LeanSignedBlock::build_minimal(
            1, 0, B256::ZERO, B256::from_slice(&[0xbe; 32])
        );
        let block_bytes = encode_gossip_data(&invalid_block);
        mock.publish(block_topic, block_bytes)
            .expect("should publish invalid block");

        sleep(Duration::from_secs(5)).await;

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
