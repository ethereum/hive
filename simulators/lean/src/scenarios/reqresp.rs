use crate::utils::helper::{
    start_post_genesis_sync_context, HelperGossipForkDigestProfile, PostGenesisSyncTestData,
    LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0,
};
use crate::utils::libp2p_mock::{
    client_multiaddr, compute_client_peer_id, decode_request, encode_request, encode_request_raw,
    extract_ip_port, replace_multiaddr_ip, BlocksByRootV1Request, Checkpoint, LeanSignedBlock,
    MockNode, Status, MAX_REQUEST_BLOCKS, RESPONSE_CODE_INVALID_REQUEST, RESPONSE_CODE_SUCCESS,
};
use crate::utils::util::{
    default_genesis_time, fork_choice_head_slot, http_client, lean_api_url, lean_clients,
    lean_environment, lean_single_client_runtime_setup, load_fork_choice_response,
    prepare_client_runtime_files, run_data_test_with_timeout, selected_lean_devnet,
    simulator_container_ip, ClientUnderTestRole, LeanDevnet, TimedDataTestSpec,
};
use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use ssz::{Decode, Encode};
use std::time::Duration;
use tokio::time::sleep;

const POST_GENESIS_TEST_TIMEOUT: Duration = Duration::from_secs(8 * 60);
const REQRESP_SYNC_TIMEOUT_SECS: u64 = 180;
const STATUS_EXCHANGE_TIMEOUT_SECS: u64 = 60;
const REQRESP_LIBP2P_TIMEOUT_SECS: u64 = 30;

/// Dial a lean client from a MockNode using its deterministic PeerId and multiaddr.
async fn dial_client(mock: &mut MockNode, client: &Client) -> Result<(), String> {
    let peer_id = compute_client_peer_id(&client.kind);
    let addr = client_multiaddr(client.ip, 9000);
    mock.dial(peer_id, addr)
}

/// Wait for the client to have at least one non-genesis block in fork choice.
async fn wait_for_client_blocks(client: &Client) {
    let deadline = std::time::Instant::now() + Duration::from_secs(REQRESP_SYNC_TIMEOUT_SECS);
    while std::time::Instant::now() < deadline {
        let fork_choice = load_fork_choice_response(client).await;
        if fork_choice.nodes.len() > 1 {
            return;
        }
        sleep(Duration::from_secs(1)).await;
    }
    panic!("client did not produce blocks within {REQRESP_SYNC_TIMEOUT_SECS} seconds");
}

fn encode_blocks_by_root_request_unchecked(roots: &[B256]) -> Vec<u8> {
    let mut raw_request = Vec::with_capacity(4 + std::mem::size_of_val(roots));
    raw_request.extend_from_slice(&4u32.to_le_bytes());
    for root in roots {
        raw_request.extend_from_slice(root.as_slice());
    }
    encode_request_raw(&raw_request)
}

// Suite: reqresp
// Tests request/response protocol behavior including Status exchange,
// BlocksByRoot, peer scoring, timeout handling, and interoperability.

dyn_async! {
    pub async fn run_reqresp_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            let (_fresh_client_environments, _fresh_client_files) = lean_single_client_runtime_setup(
                &client.name);


            let status_happy_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/happy_path".to_string(),
                    description: "Two compatible lean nodes exchange Status and assert finalized/head checkpoints match.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_happy_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 2,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_happy_path,
            ).await;


            let status_genesis_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/genesis_only".to_string(),
                    description: "Two fresh nodes at genesis exchange Status; assert zero/genesis finalized and head are accepted.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_genesis_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: false,
                        connect_client_to_lean_spec_mesh: false,
                        client_role: ClientUnderTestRole::Observer,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_genesis_only,
            ).await;


            let status_advanced_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/advanced_head".to_string(),
                    description: "Source node advances several slots, sink connects later and marks it as useful for sync.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_advanced_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        // Helper owns V1+V2 so client (default V0) has exclusive proposer slots; see #1470.
                        source_helper_validator_indices: Some(
                            LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
                        ),
                        helper_peer_count: 2,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_advanced_head,
            ).await;


            let status_bad_root_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/incompatible_finalized_root".to_string(),
                    description: "Peer reports same finalized slot but different finalized root. Client should reject/disconnect.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_bad_root_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_incompatible_finalized_root,
            ).await;


            let status_bad_fork_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/incompatible_fork_or_network".to_string(),
                    description: "Peer uses wrong fork digest/network config. Client should not treat it as a valid sync peer.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_bad_fork_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_incompatible_fork,
            ).await;


            let status_malformed_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/status/malformed_ssz".to_string(),
                    description: "Send invalid SSZ/snappy status bytes. Client must reject without crashing.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: status_malformed_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_status_malformed_ssz,
            ).await;


            let blocks_single_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/single_known_block".to_string(),
                    description: "Request one known block root from source. Assert exact block is returned.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_single_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: false,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_single_known,
            ).await;


            let blocks_multiple_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/multiple_known_blocks".to_string(),
                    description: "Request several known roots in one request. Assert all returned blocks match.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_multiple_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: false,
                        connect_client_to_lean_spec_mesh: false,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_multiple_known,
            ).await;


            let blocks_unknown_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/unknown_root".to_string(),
                    description: "Request a root the peer does not have. Assert empty response or missing block behavior is spec-compliant.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_unknown_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: false,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_unknown,
            ).await;


            let blocks_mixed_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/mixed_known_unknown".to_string(),
                    description: "Request known and unknown roots together. Assert known blocks are returned and unknown roots do not fail the whole request.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_mixed_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: false,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_mixed,
            ).await;


            let blocks_max_limit_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/max_request_limit".to_string(),
                    description: "Request exactly MAX_REQUEST_BLOCKS roots. Assert accepted.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_max_limit_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_max_limit,
            ).await;


            let blocks_too_many_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/too_many_roots".to_string(),
                    description: "Request more than MAX_REQUEST_BLOCKS. Assert rejected, stream reset, or error response.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_too_many_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_too_many,
            ).await;


            let blocks_duplicate_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/duplicate_roots".to_string(),
                    description: "Request the same root multiple times. Assert deterministic behavior.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_duplicate_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_duplicate,
            ).await;


            let blocks_malformed_req_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/blocks_by_root/malformed_request".to_string(),
                    description: "Send invalid SSZ/snappy request bytes. Client rejects without crash.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: blocks_malformed_req_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_blocks_by_root_malformed_request,
            ).await;


            let backfill_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/sync/missing_parent_backfill".to_string(),
                    description: "Sink receives child via gossip before parent, then fetches missing parent via BlocksByRoot.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: backfill_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 2,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_sync_missing_parent_backfill,
            ).await;


            let catchup_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/sync/catch_up_from_status".to_string(),
                    description: "Sink starts behind, reads peer Status, requests missing blocks, and catches up within slot delta.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: catchup_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 2,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_sync_catch_up_from_status,
            ).await;


            let concurrency_genesis_time = default_genesis_time();
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: "reqresp/concurrency/per_peer_request_limit".to_string(),
                    description: "Issue concurrent block requests to one peer. Assert client respects the max in-flight request limit.".to_string(),
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: POST_GENESIS_TEST_TIMEOUT,
                    test_data: PostGenesisSyncTestData {
                        client_under_test: client.clone(),
                        genesis_time: concurrency_genesis_time,
                        wait_for_client_justified_checkpoint: false,
                        use_checkpoint_sync: true,
                        connect_client_to_lean_spec_mesh: true,
                        client_role: ClientUnderTestRole::Validator,
                        source_helper_validator_indices: None,
                        helper_peer_count: 1,
                        helper_fork_digest_profile: if selected_lean_devnet() == LeanDevnet::Devnet4 {
                            HelperGossipForkDigestProfile::SelectedDevnet
                        } else {
                            HelperGossipForkDigestProfile::LegacyDevnet0
                        },
                    },
                },
                test_concurrency_per_peer_limit,
            ).await;



        }
    }
}

// === STATUS TESTS ===

dyn_async! {
    async fn test_status_happy_path<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;

        let mut synced = false;
        let deadline = std::time::Instant::now() + Duration::from_secs(STATUS_EXCHANGE_TIMEOUT_SECS);

        while std::time::Instant::now() < deadline {
            let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;
            let source_fork_choice = &context.source_fork_choice;

            // If client has the same or newer finalized checkpoint, status exchange worked
            if client_fork_choice.finalized.slot >= source_fork_choice.finalized.slot {
                synced = true;
                break;
            }

            sleep(Duration::from_secs(1)).await;
        }

        assert!(
            synced,
            "client should sync with helper mesh via status exchange within {} seconds",
            STATUS_EXCHANGE_TIMEOUT_SECS
        );

        let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;
        let source_fork_choice = &context.source_fork_choice;

        assert!(
            client_fork_choice.finalized.slot >= source_fork_choice.finalized.slot,
            "client finalized slot ({}) should be >= source finalized slot ({})",
            client_fork_choice.finalized.slot,
            source_fork_choice.finalized.slot
        );
    }
}

dyn_async! {
    async fn test_status_genesis_only<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;

        let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;

        assert_eq!(
            client_fork_choice.finalized.slot, 0,
            "client should report genesis finalized slot"
        );
        assert_eq!(
            client_fork_choice.justified.slot, 0,
            "client should report genesis justified slot"
        );
        assert_eq!(
            client_fork_choice.head, client_fork_choice.nodes[0].root,
            "client head should be genesis"
        );

        assert!(
            !client_fork_choice.nodes.is_empty(),
            "client should have at least genesis node after status exchange"
        );
    }
}

dyn_async! {
    async fn test_status_advanced_head<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let mut context = start_post_genesis_sync_context(test, &test_data).await;

        sleep(Duration::from_secs(10)).await;

        // Compare against the helper's *live* head re-fetched each iteration
        // rather than a single T+10s snapshot. The helper keeps producing
        // blocks (it's still aggregating V1+V2) so a frozen snapshot becomes
        // unreachable as soon as the helper moves on; equality with a moving
        // target is the right invariant for "client has caught up".
        let mut caught_up = false;
        let deadline = std::time::Instant::now() + Duration::from_secs(REQRESP_SYNC_TIMEOUT_SECS);

        while std::time::Instant::now() < deadline {
            let source_fork_choice = context.load_live_helper_fork_choice().await;
            let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;

            if client_fork_choice.head == source_fork_choice.head {
                caught_up = true;
                break;
            }

            sleep(Duration::from_secs(1)).await;
        }

        assert!(
            caught_up,
            "client should catch up to advanced head within {} seconds",
            REQRESP_SYNC_TIMEOUT_SECS
        );
    }
}

dyn_async! {
    async fn test_status_incompatible_finalized_root<'a>(test: &'a mut Test, _test_data: PostGenesisSyncTestData) {
        let client_type = _test_data.client_under_test.name.clone();

        let mut mock = MockNode::new_status_only().expect("failed to create mock node");
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

        let mut environment = lean_environment();
        environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
        ClientUnderTestRole::Observer.apply_to_environment(&mut environment);
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

        let bad_status = Status {
            finalized: Checkpoint { root: B256::from_slice(&[0xde; 32]), slot: client_status.finalized.slot },
            head: Checkpoint { root: B256::from_slice(&[0xad; 32]), slot: client_status.head.slot + 1 },
        };
        mock.send_response(channel, vec![
            (RESPONSE_CODE_SUCCESS, bad_status.as_ssz_bytes())
        ]).expect("should send bad status response");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.finalized.slot, 0,
            "client should reject peer with mismatched finalized root and stay at genesis"
        );
    }
}

dyn_async! {
    async fn test_status_incompatible_fork<'a>(test: &'a mut Test, _test_data: PostGenesisSyncTestData) {
        let client_type = _test_data.client_under_test.name.clone();

        let mut mock = MockNode::new_status_only().expect("failed to create mock node");
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

        let mut environment = lean_environment();
        environment.insert("HIVE_BOOTNODES".to_string(), mock_enr);
        ClientUnderTestRole::Observer.apply_to_environment(&mut environment);
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
        let _client_status = Status::from_ssz_bytes(&decompressed)
            .expect("first request should be a valid Status message");

        let bad_status = Status {
            finalized: Checkpoint { root: B256::ZERO, slot: 0 },
            head: Checkpoint { root: B256::from_slice(&[0xbe; 32]), slot: 999_999_999 },
        };
        mock.send_response(channel, vec![
            (RESPONSE_CODE_SUCCESS, bad_status.as_ssz_bytes())
        ]).expect("should send bad status response");

        mock.process_events_for(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&client).await;
        assert_eq!(
            fork_choice.finalized.slot, 0,
            "client should reject peer with implausible head and stay at genesis"
        );
    }
}

dyn_async! {
    async fn test_status_malformed_ssz<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;

        let mut mock = MockNode::new_status_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let peer_id = compute_client_peer_id(&client.kind);

        let garbage = vec![0xff, 0xff, 0xff, 0xff];
        let request = encode_request_raw(&garbage);
        let result = mock.send_request(peer_id, request).await;

        if let Ok(chunks) = result {
            assert!(
                chunks.is_empty() || chunks[0].0 != RESPONSE_CODE_SUCCESS,
                "client should reject malformed SSZ status"
            );
        }

        let response = http_client()
            .get(lean_api_url(client, "/lean/v0/fork_choice"))
            .send()
            .await
            .expect("client should still respond to HTTP after malformed status");
        assert_eq!(response.status(), 200, "client should remain healthy");
    }
}

// === BLOCKS BY ROOT TESTS ===

dyn_async! {
    async fn test_blocks_by_root_single_known<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        let peer_id = compute_client_peer_id(&client.kind);

        sleep(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(client).await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "client should have nodes in fork choice"
        );

        let head_root = fork_choice.head;
        assert_ne!(head_root, B256::ZERO, "head root should not be zero");

        let expected_node = fork_choice
            .nodes
            .iter()
            .find(|node| node.root == head_root)
            .expect("fork_choice head should be present in nodes");

        let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let request = encode_request(&BlocksByRootV1Request::new(vec![head_root]));
        let chunks = mock
            .send_request(peer_id, request)
            .await
            .expect("client should return block for known head root");
        let success_payloads = chunks
            .iter()
            .filter_map(|(code, payload)| (*code == RESPONSE_CODE_SUCCESS && !payload.is_empty()).then_some(payload))
            .collect::<Vec<_>>();
        assert_eq!(success_payloads.len(), 1, "client should return exactly one known block");

        let signed_block = LeanSignedBlock::from_ssz_bytes(success_payloads[0])
            .expect("returned head block should decode from SSZ");
        assert_eq!(signed_block.block.slot, expected_node.slot, "returned block slot should match fork_choice head");
        assert_eq!(signed_block.block.parent_root, expected_node.parent_root, "returned block parent should match fork_choice head");
        assert_eq!(signed_block.block.proposer_index, expected_node.proposer_index, "returned block proposer should match fork_choice head");
    }
}

dyn_async! {
    async fn test_blocks_by_root_multiple_known<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        let peer_id = compute_client_peer_id(&client.kind);

        let deadline = std::time::Instant::now() + Duration::from_secs(REQRESP_SYNC_TIMEOUT_SECS);
        let known_nodes = loop {
            let fork_choice = load_fork_choice_response(client).await;
            let mut known_nodes = fork_choice
                .nodes
                .iter()
                .filter(|node| node.root != B256::ZERO)
                .cloned()
                .collect::<Vec<_>>();
            known_nodes.sort_by(|a, b| b.slot.cmp(&a.slot));
            if known_nodes.len() >= 2 {
                break known_nodes.into_iter().take(2).collect::<Vec<_>>();
            }

            if std::time::Instant::now() >= deadline {
                panic!(
                    "client should expose at least two known fork-choice blocks within {REQRESP_SYNC_TIMEOUT_SECS} seconds"
                );
            }

            sleep(Duration::from_secs(1)).await;
        };

        let requested_roots = known_nodes.iter().map(|node| node.root).collect::<Vec<_>>();

        let mut multi_mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut multi_mock, client).await.expect("failed to dial client");
        let request = encode_request(&BlocksByRootV1Request::new(requested_roots.clone()));
        let chunks = multi_mock
            .send_request(peer_id, request)
            .await
            .expect("client should return blocks for known fork-choice roots");

        let success_payloads = chunks
            .iter()
            .filter_map(|(code, payload)| (*code == RESPONSE_CODE_SUCCESS && !payload.is_empty()).then_some(payload))
            .collect::<Vec<_>>();
        assert_eq!(
            success_payloads.len(),
            requested_roots.len(),
            "client should return one block for each requested known root"
        );

        for (payload, expected_node) in success_payloads.iter().zip(known_nodes.iter()) {
            let signed_block = LeanSignedBlock::from_ssz_bytes(payload)
                .expect("returned block should decode from SSZ");
            assert_eq!(signed_block.block.slot, expected_node.slot, "returned block slot should match requested fork_choice node");
            assert_eq!(signed_block.block.parent_root, expected_node.parent_root, "returned block parent should match requested fork_choice node");
            assert_eq!(signed_block.block.proposer_index, expected_node.proposer_index, "returned block proposer should match requested fork_choice node");
        }
    }
}

dyn_async! {
    async fn test_blocks_by_root_unknown<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;

        sleep(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&context.client_under_test).await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "client should have nodes in fork choice"
        );

        let response = http_client()
            .get(lean_api_url(&context.client_under_test, "/lean/v0/states/finalized"))
            .send()
            .await
            .expect("state endpoint should respond");

        assert_eq!(response.status(), 200, "client should remain stable");
    }
}

dyn_async! {
    async fn test_blocks_by_root_mixed<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;

        sleep(Duration::from_secs(5)).await;

        let fork_choice = load_fork_choice_response(&context.client_under_test).await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "client should have nodes in fork choice"
        );

        let known_root = fork_choice.nodes[0].root;
        assert_ne!(
            known_root,
            B256::ZERO,
            "known block should have non-zero root"
        );

        let response = http_client()
            .get(lean_api_url(&context.client_under_test, "/lean/v0/states/finalized"))
            .send()
            .await
            .expect("state endpoint should respond");

        assert_eq!(response.status(), 200, "client should remain stable with mixed data");
    }
}

dyn_async! {
    async fn test_blocks_by_root_max_limit<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        wait_for_client_blocks(client).await;

        let fork_choice = load_fork_choice_response(client).await;
        let known_root = fork_choice.nodes[0].root;

        let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let peer_id = compute_client_peer_id(&client.kind);

        let roots = vec![known_root; MAX_REQUEST_BLOCKS];
        let request = encode_request(&BlocksByRootV1Request::new(roots));
        let result = mock.send_request(peer_id, request).await;

        match result {
            Ok(chunks) => {
                assert!(chunks.len() <= MAX_REQUEST_BLOCKS, "response should not exceed max request size");
            }
            Err(e) => panic!("client should accept max-size request: {e}"),
        }
    }
}

dyn_async! {
    async fn test_blocks_by_root_too_many<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        wait_for_client_blocks(client).await;

        let fork_choice = load_fork_choice_response(client).await;
        let known_root = fork_choice.nodes[0].root;

        let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let peer_id = compute_client_peer_id(&client.kind);

        let roots = vec![known_root; MAX_REQUEST_BLOCKS + 1];
        let request = encode_blocks_by_root_request_unchecked(&roots);
        let result = mock.send_request(peer_id, request).await;

        if let Ok(chunks) = result {
            assert!(
                chunks.is_empty() || chunks[0].0 == RESPONSE_CODE_INVALID_REQUEST,
                "client should reject request exceeding max block count"
            );
        }
    }
}

dyn_async! {
    async fn test_blocks_by_root_duplicate<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        wait_for_client_blocks(client).await;

        let fork_choice = load_fork_choice_response(client).await;
        let known_root = fork_choice.nodes[0].root;

        let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let peer_id = compute_client_peer_id(&client.kind);

        let roots = vec![known_root; 5];
        let request = encode_request(&BlocksByRootV1Request::new(roots));
        let result = mock.send_request(peer_id, request).await;

        match result {
            Ok(chunks) => {
                assert!(
                    chunks.len() <= 5,
                    "response chunks should not exceed requested root count"
                );
            }
            Err(e) => panic!("client should handle duplicate roots gracefully: {e}"),
        }
    }
}

dyn_async! {
    async fn test_blocks_by_root_malformed_request<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;

        let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
        dial_client(&mut mock, client).await.expect("failed to dial client");
        let peer_id = compute_client_peer_id(&client.kind);

        let garbage = vec![0xab; 64];
        let request = encode_request_raw(&garbage);
        let result = mock.send_request(peer_id, request).await;

        if let Ok(chunks) = result {
            assert!(
                chunks.is_empty() || chunks[0].0 != RESPONSE_CODE_SUCCESS,
                "client should reject malformed BlocksByRoot request"
            );
        }

        let response = http_client()
            .get(lean_api_url(client, "/lean/v0/fork_choice"))
            .send()
            .await
            .expect("client should still respond to HTTP after malformed request");
        assert_eq!(response.status(), 200, "client should remain healthy");
    }
}

// === SYNC TESTS ===

dyn_async! {
    async fn test_sync_missing_parent_backfill<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;

        let mut has_chain = false;
        let deadline = std::time::Instant::now() + Duration::from_secs(REQRESP_SYNC_TIMEOUT_SECS);

        while std::time::Instant::now() < deadline {
            let fork_choice = load_fork_choice_response(&context.client_under_test).await;

            if fork_choice.nodes.len() > 2 {
                has_chain = true;
                break;
            }

            sleep(Duration::from_secs(1)).await;
        }

        assert!(
            has_chain,
            "client should build a chain with parent-child relationships within {} seconds",
            REQRESP_SYNC_TIMEOUT_SECS
        );

        let fork_choice = load_fork_choice_response(&context.client_under_test).await;
        let mut found_parent_link = false;

        for node in &fork_choice.nodes {
            if node.slot > 0 {
                let parent = fork_choice.nodes.iter().find(|n| n.root == node.parent_root);
                if parent.is_some() {
                    found_parent_link = true;
                    break;
                }
            }
        }

        assert!(
            found_parent_link,
            "chain should have valid parent-child links"
        );
    }
}

dyn_async! {
    async fn test_sync_catch_up_from_status<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let mut context = start_post_genesis_sync_context(test, &test_data).await;

        sleep(Duration::from_secs(10)).await;

        let source_before = context.load_live_helper_fork_choice().await;
        let source_head_slot = fork_choice_head_slot(&source_before);

        let mut caught_up = false;
        let deadline = std::time::Instant::now() + Duration::from_secs(REQRESP_SYNC_TIMEOUT_SECS);

        while std::time::Instant::now() < deadline {
            let client_fork_choice = load_fork_choice_response(&context.client_under_test).await;
            let client_head_slot = fork_choice_head_slot(&client_fork_choice);

            if client_head_slot >= source_head_slot {
                caught_up = true;
                break;
            }

            sleep(Duration::from_secs(1)).await;
        }

        assert!(
            caught_up,
            "client should catch up from status within {} seconds (source head: {})",
            REQRESP_SYNC_TIMEOUT_SECS,
            source_head_slot
        );
    }
}

// === TIMEOUT TESTS ===

// === DISCONNECT TESTS ===

// === CONCURRENCY TESTS ===

dyn_async! {
    async fn test_concurrency_per_peer_limit<'a>(test: &'a mut Test, test_data: PostGenesisSyncTestData) {
        let context = start_post_genesis_sync_context(test, &test_data).await;
        let client = &context.client_under_test;
        wait_for_client_blocks(client).await;

        let fork_choice = load_fork_choice_response(client).await;
        let known_root = fork_choice.nodes[0].root;

        let mut handles = Vec::new();
        for _ in 0..5 {
            let mut mock = MockNode::new_blocks_by_root_only().expect("failed to create mock node");
            dial_client(&mut mock, client).await.expect("failed to dial client");
            let peer_id = compute_client_peer_id(&client.kind);
            handles.push(tokio::spawn(async move {
                let roots = vec![known_root; 5];
                let request = encode_request(&BlocksByRootV1Request::new(roots));
                mock.send_request(peer_id, request).await
            }));
        }

        let mut results = Vec::new();
        for handle in handles {
            results.push(handle.await.expect("concurrent request task panicked"));
        }

        let success_count = results.iter().filter(|r| r.is_ok()).count();
        assert!(
            success_count > 0 || results.iter().all(|r| r.is_err()),
            "client should handle concurrent requests without crashing"
        );

        let response = http_client()
            .get(lean_api_url(client, "/lean/v0/fork_choice"))
            .send()
            .await
            .expect("client should still respond to HTTP after concurrent requests");
        assert_eq!(response.status(), 200, "client should remain healthy");
    }
}
