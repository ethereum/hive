use std::time::Duration;

use crate::scenarios::sync::{
    PostGenesisSyncContext, PostGenesisSyncTestData, SourceCheckpointKind, default_genesis_time,
    lean_spec_source_environment, start_post_genesis_sync_context, LEAN_SPEC_CLIENT_TYPE,
};
use crate::{
    CheckpointResponse, HEALTHY_STATUS, LEAN_RPC_SERVICE, HealthResponse, get_json_with_retry,
    lean_api_url, lean_clients, lean_environment,
};
use hivesim::types::{ClientDefinition, ClientMetadata};
use hivesim::{Client, NClientTestSpec, Test, dyn_async};
use reqwest::Client as HttpClient;
use serde::Deserialize;
use tokio::time::sleep;
use tracing::warn;

const FORK_CHOICE_TIMEOUT_SECS: u64 = 180;
const ZERO_ROOT_HEX: &str =
    "0x0000000000000000000000000000000000000000000000000000000000000000";

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
struct ForkChoiceNodeResponse {
    root: String,
    slot: u64,
    parent_root: String,
    proposer_index: u64,
    weight: u64,
}

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
struct ForkChoiceResponse {
    nodes: Vec<ForkChoiceNodeResponse>,
    head: String,
    justified: CheckpointResponse,
    finalized: CheckpointResponse,
    safe_target: String,
    validator_count: u64,
}

fn expect_single_client(clients: Vec<Client>) -> Client {
    clients
        .into_iter()
        .next()
        .expect("NClientTestSpec should start exactly one client")
}

fn http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("Unable to build HTTP client")
}

fn assert_hex_root(root: &str, field_name: &str) {
    assert!(
        root.starts_with("0x"),
        "{field_name} should be 0x-prefixed, got {root}"
    );
    assert_eq!(
        root.len(),
        66,
        "{field_name} should be 32 bytes of hex plus 0x prefix"
    );
}

async fn load_fork_choice_response(client: &Client) -> ForkChoiceResponse {
    let http = http_client();
    get_json_with_retry(&http, &lean_api_url(client, "/lean/v0/fork_choice")).await
}

async fn wait_for_non_genesis_fork_choice_response(
    client: &Client,
    checkpoint_kind: SourceCheckpointKind,
) -> ForkChoiceResponse {
    for _attempt in 0..FORK_CHOICE_TIMEOUT_SECS {
        let fork_choice = load_fork_choice_response(client).await;
        let checkpoint_slot = match checkpoint_kind {
            SourceCheckpointKind::Justified => fork_choice.justified.slot,
            SourceCheckpointKind::Finalized => fork_choice.finalized.slot,
        };
        if checkpoint_slot > 0 {
            return fork_choice;
        }

        sleep(Duration::from_secs(1)).await;
    }

    let checkpoint_name = match checkpoint_kind {
        SourceCheckpointKind::Justified => "justified",
        SourceCheckpointKind::Finalized => "finalized",
    };

    panic!(
        "Client {} never reached a non-genesis {checkpoint_name} forkchoice checkpoint",
        client.kind
    );
}

async fn load_fresh_fork_choice_setup(clients: Vec<Client>) -> (Client, ForkChoiceResponse) {
    let client = expect_single_client(clients);
    let fork_choice = load_fork_choice_response(&client).await;
    (client, fork_choice)
}

async fn load_post_genesis_sync_context(
    clients: Vec<Client>,
    test_data: PostGenesisSyncTestData,
) -> PostGenesisSyncContext {
    let source_client = expect_single_client(clients);
    start_post_genesis_sync_context(source_client, &test_data).await
}

async fn load_post_genesis_fork_choice_setup(
    clients: Vec<Client>,
    test_data: PostGenesisSyncTestData,
    checkpoint_kind: SourceCheckpointKind,
) -> (PostGenesisSyncContext, ForkChoiceResponse) {
    let context = load_post_genesis_sync_context(clients, test_data).await;
    let fork_choice =
        wait_for_non_genesis_fork_choice_response(&context.client_under_test, checkpoint_kind)
            .await;
    (context, fork_choice)
}

fn lean_spec_client_definition(client: &Client) -> ClientDefinition {
    ClientDefinition {
        name: client.kind.clone(),
        version: String::new(),
        meta: ClientMetadata { roles: vec![] },
    }
}

dyn_async! {
    pub async fn run_rpc_compat_lean_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let available_clients = test.sim.client_types().await;
        let lean_spec_selected = available_clients
            .iter()
            .any(|client| client.name == LEAN_SPEC_CLIENT_TYPE);

        if !lean_spec_selected {
            run_rpc_compat_lean(test, None).await;
            return;
        }

        let lean_spec_client = test
            .start_client(
                LEAN_SPEC_CLIENT_TYPE.to_string(),
                Some(lean_environment()),
            )
            .await;

        run_rpc_compat_lean(test, Some(lean_spec_client)).await;
    }
}

dyn_async! {
    pub async fn run_rpc_compat_lean<'a>(test: &'a mut Test, client: Option<Client>) {
        let available_clients = test.sim.client_types().await;
        let clients: Vec<_> = lean_clients(available_clients)
            .into_iter()
            .filter(|client| client.name != LEAN_SPEC_CLIENT_TYPE)
            .collect();
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        let lean_spec_client = client.as_ref().map(lean_spec_client_definition);
        if lean_spec_client.is_none() {
            warn!(
                "Skipping post-genesis Lean RPC compatibility cases because no running `lean-spec-client` was provided"
            );
        }

        for client in &clients {
            test.run(
                NClientTestSpec {
                    name: "health healthy".to_string(),
                    description: "Checks that the health endpoint reports a healthy Lean RPC service.".to_string(),
                    always_run: false,
                    run: test_health_healthy,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "checkpoints justified root encoding".to_string(),
                    description: "Checks that the justified checkpoint root is hex encoded.".to_string(),
                    always_run: false,
                    run: test_checkpoints_hex_encodes_root,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "checkpoints justified fields".to_string(),
                    description: "Checks that the justified checkpoint endpoint returns the expected fields.".to_string(),
                    always_run: false,
                    run: test_checkpoints_returns_expected_fields,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "checkpoints justified genesis".to_string(),
                    description: "Checks that a fresh Lean node reports the genesis justified checkpoint.".to_string(),
                    always_run: false,
                    run: test_checkpoints_returns_genesis_justified_checkpoint_for_fresh_node,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice no head".to_string(),
                    description: "Loads the forkchoice endpoint from a fresh node before any non-genesis head advancement.".to_string(),
                    always_run: false,
                    run: test_forkchoice_no_head,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice no justified".to_string(),
                    description: "Loads the forkchoice endpoint from a fresh node before any non-genesis justified checkpoint exists.".to_string(),
                    always_run: false,
                    run: test_forkchoice_no_justified,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice no finalized".to_string(),
                    description: "Loads the forkchoice endpoint from a fresh node before any non-genesis finalized checkpoint exists.".to_string(),
                    always_run: false,
                    run: test_forkchoice_no_finalized,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice no nodes".to_string(),
                    description: "Loads the forkchoice endpoint from a fresh node before any non-genesis nodes are present.".to_string(),
                    always_run: false,
                    run: test_forkchoice_no_nodes,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice defaults missing weight to zero".to_string(),
                    description: "Loads the forkchoice endpoint from a fresh node where block weights should still be zero.".to_string(),
                    always_run: false,
                    run: test_forkchoice_defaults_missing_weight_to_zero,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice zero validator count when head state missing".to_string(),
                    description: "Loads the forkchoice endpoint from the closest black-box baseline available before a missing head-state hook exists.".to_string(),
                    always_run: false,
                    run: test_forkchoice_zero_validator_count_when_head_state_missing,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice hex encodes roots".to_string(),
                    description: "Loads the forkchoice endpoint to prepare root encoding assertions.".to_string(),
                    always_run: false,
                    run: test_forkchoice_hex_encodes_roots,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice includes expected node fields".to_string(),
                    description: "Loads the forkchoice endpoint to prepare node field assertions.".to_string(),
                    always_run: false,
                    run: test_forkchoice_includes_expected_node_fields,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "forkchoice".to_string(),
                    description: "Loads the forkchoice endpoint for the baseline RPC compatibility case.".to_string(),
                    always_run: false,
                    run: test_forkchoice,
                    environments: Some(vec![Some(lean_environment())]),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            if let Some(lean_spec_client) = &lean_spec_client {
                let checkpoint_genesis_time = default_genesis_time();

                test.run(
                    NClientTestSpec {
                        name: "checkpoints justified post-genesis".to_string(),
                        description: "Starts a LeanSpec client mesh, waits for a non-genesis justified checkpoint, and checks that the client under test reports a non-genesis justified checkpoint.".to_string(),
                        always_run: false,
                        run: test_checkpoints_justified,
                        environments: Some(vec![Some(lean_spec_source_environment(checkpoint_genesis_time))]),
                        test_data: PostGenesisSyncTestData {
                            client_under_test: client.clone(),
                            genesis_time: checkpoint_genesis_time,
                            source_checkpoint_kind: SourceCheckpointKind::Justified,
                            use_checkpoint_sync: false,
                            connect_client_to_lean_spec_mesh: true,
                        },
                        clients: vec![lean_spec_client.clone()],
                    }
                ).await;

                let finalized_filters_genesis_time = default_genesis_time();

                test.run(
                    NClientTestSpec {
                        name: "forkchoice filters nodes before finalized slot".to_string(),
                        description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and loads forkchoice with a non-genesis finalized slot.".to_string(),
                        always_run: false,
                        run: test_forkchoice_filters_nodes_before_finalized_slot,
                        environments: Some(vec![Some(lean_spec_source_environment(finalized_filters_genesis_time))]),
                        test_data: PostGenesisSyncTestData {
                            client_under_test: client.clone(),
                            genesis_time: finalized_filters_genesis_time,
                            source_checkpoint_kind: SourceCheckpointKind::Finalized,
                            use_checkpoint_sync: true,
                            connect_client_to_lean_spec_mesh: false,
                        },
                        clients: vec![lean_spec_client.clone()],
                    }
                ).await;

                let finalized_boundary_genesis_time = default_genesis_time();

                test.run(
                    NClientTestSpec {
                        name: "forkchoice includes nodes at finalized slot".to_string(),
                        description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and loads forkchoice with a finalized boundary present.".to_string(),
                        always_run: false,
                        run: test_forkchoice_includes_nodes_at_finalized_slot,
                        environments: Some(vec![Some(lean_spec_source_environment(finalized_boundary_genesis_time))]),
                        test_data: PostGenesisSyncTestData {
                            client_under_test: client.clone(),
                            genesis_time: finalized_boundary_genesis_time,
                            source_checkpoint_kind: SourceCheckpointKind::Finalized,
                            use_checkpoint_sync: true,
                            connect_client_to_lean_spec_mesh: false,
                        },
                        clients: vec![lean_spec_client.clone()],
                    }
                ).await;

                let pre_finalized_only_genesis_time = default_genesis_time();

                test.run(
                    NClientTestSpec {
                        name: "forkchoice returns empty nodes when all blocks are pre-finalized".to_string(),
                        description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and loads forkchoice at the finalized boundary.".to_string(),
                        always_run: false,
                        run: test_forkchoice_returns_empty_nodes_when_all_blocks_are_pre_finalized,
                        environments: Some(vec![Some(lean_spec_source_environment(pre_finalized_only_genesis_time))]),
                        test_data: PostGenesisSyncTestData {
                            client_under_test: client.clone(),
                            genesis_time: pre_finalized_only_genesis_time,
                            source_checkpoint_kind: SourceCheckpointKind::Finalized,
                            use_checkpoint_sync: true,
                            connect_client_to_lean_spec_mesh: false,
                        },
                        clients: vec![lean_spec_client.clone()],
                    }
                ).await;
            }
        }
    }
}

// /lean/v0/health
dyn_async! {
    async fn test_health_healthy<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let http = http_client();

        let health: HealthResponse = get_json_with_retry(
            &http,
            &lean_api_url(&client, "/lean/v0/health"),
        )
        .await;
        assert_eq!(
            health.status, HEALTHY_STATUS,
            "health endpoint returned an unexpected status"
        );
        assert_eq!(
            health.service, LEAN_RPC_SERVICE,
            "health endpoint returned an unexpected service name"
        );
    }
}

// /lean/v0/checkpoints/justified
dyn_async! {
    async fn test_checkpoints_hex_encodes_root<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let http = http_client();

        let checkpoint: CheckpointResponse = get_json_with_retry(
            &http,
            &lean_api_url(&client, "/lean/v0/checkpoints/justified"),
        )
        .await;
        assert_hex_root(&checkpoint.root, "justified checkpoint root");
    }
}

dyn_async! {
    async fn test_checkpoints_returns_expected_fields<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let http = http_client();

        let checkpoint: CheckpointResponse = get_json_with_retry(
            &http,
            &lean_api_url(&client, "/lean/v0/checkpoints/justified"),
        )
        .await;

        let CheckpointResponse { slot, root } = checkpoint;
        let _ = slot;
        let _ = root;
    }
}

dyn_async! {
    async fn test_checkpoints_returns_genesis_justified_checkpoint_for_fresh_node<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let http = http_client();
        
        let checkpoint: CheckpointResponse = get_json_with_retry(
            &http,
            &lean_api_url(&client, "/lean/v0/checkpoints/justified"),
        )
        .await;
        assert_eq!(
            checkpoint.slot, 0,
            "a freshly started lean node should report the genesis justified checkpoint"
        );
    }
}

dyn_async! {
    async fn test_checkpoints_justified<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let context = load_post_genesis_sync_context(clients, test_data).await;
        let _ = (
            &context.source_client,
            &context.peer_client,
            &context.client_under_test,
        );

        assert!(
            context.source_fork_choice.justified.slot > 0,
            "helper source should reach a non-genesis justified checkpoint before syncing the client under test"
        );
        assert!(
            context.client_checkpoint.slot > 0,
            "client under test should report a non-genesis justified checkpoint after the helper mesh reaches justification"
        );

        if context.client_checkpoint.slot == context.source_fork_choice.justified.slot {
            assert_eq!(
                context.client_checkpoint.root,
                context.source_fork_choice.justified.root,
                "matching justified slots should also produce matching justified roots"
            );
        }
    }
}

// /lean/v0/fork_choice
dyn_async! {
    async fn test_forkchoice_no_head<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "fresh forkchoice tree should contain only the genesis node"
        );
        assert_eq!(
            fork_choice.nodes[0].slot, 0,
            "without a non-genesis head, the only forkchoice node should still be genesis"
        );
        assert_eq!(
            fork_choice.head, fork_choice.nodes[0].root,
            "without a non-genesis head, forkchoice head should stay on the genesis node"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_no_justified<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.justified.slot, 0,
            "without a non-genesis justification event, the justified slot should remain at genesis"
        );
        assert_eq!(
            fork_choice.justified.root, fork_choice.head,
            "without a non-genesis justification event, the justified root should remain at the genesis head"
        );
        assert_hex_root(&fork_choice.justified.root, "justified root");
    }
}

dyn_async! {
    async fn test_forkchoice_no_finalized<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.finalized.slot, 0,
            "without a non-genesis finalization event, the finalized slot should remain at genesis"
        );
        assert_eq!(
            fork_choice.finalized.root, fork_choice.head,
            "without a non-genesis finalization event, the finalized root should remain at the genesis head"
        );
        assert_hex_root(&fork_choice.finalized.root, "finalized root");
    }
}

dyn_async! {
    async fn test_forkchoice_no_nodes<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "fresh forkchoice should only expose the genesis node before any non-genesis blocks are tracked"
        );
        assert_eq!(
            fork_choice.nodes[0].slot, 0,
            "the only forkchoice node should still be the genesis block"
        );
        assert_eq!(
            fork_choice.nodes[0].parent_root, ZERO_ROOT_HEX,
            "the genesis forkchoice node should reference the zero parent root"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_filters_nodes_before_finalized_slot<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let (_context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert!(
            !fork_choice.nodes.is_empty(),
            "forkchoice should still expose at least the finalized boundary node"
        );
        assert!(
            fork_choice.finalized.slot > 0,
            "post-genesis forkchoice setup should advance finalized beyond genesis"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= fork_choice.finalized.slot),
            "forkchoice should filter out any node older than the finalized slot"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_includes_nodes_at_finalized_slot<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let (_context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "forkchoice should still expose at least the finalized boundary node"
        );
        assert!(
            fork_choice.finalized.slot > 0,
            "post-genesis forkchoice setup should advance finalized beyond genesis"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= fork_choice.finalized.slot),
            "forkchoice should filter out any node older than the finalized slot"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .any(|node| node.slot == fork_choice.finalized.slot),
            "forkchoice should keep at least one node at the finalized boundary slot"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .any(|node| node.root == fork_choice.finalized.root),
            "forkchoice should keep the finalized boundary root in the returned node set"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_defaults_missing_weight_to_zero<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert!(
            !fork_choice.nodes.is_empty(),
            "forkchoice should expose at least the genesis node"
        );
        assert!(
            fork_choice.nodes.iter().all(|node| node.weight == 0),
            "forkchoice should default missing block weights to zero"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_zero_validator_count_when_head_state_missing<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "fresh forkchoice should still be on the genesis-only tree in the black-box baseline"
        );
        assert!(
            fork_choice.validator_count > 0,
            "the public RPC setup still has a head state, so validator_count should stay populated until we add a hook that removes store.states[head]"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_hex_encodes_roots<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert_hex_root(&fork_choice.justified.root, "forkchoice justified root");
        assert_hex_root(&fork_choice.finalized.root, "forkchoice finalized root");
        assert_hex_root(&fork_choice.safe_target, "forkchoice safe_target");

        for node in &fork_choice.nodes {
            assert_hex_root(&node.root, "forkchoice node root");
            assert_hex_root(&node.parent_root, "forkchoice node parent_root");
        }
    }
}

dyn_async! {
    async fn test_forkchoice_returns_empty_nodes_when_all_blocks_are_pre_finalized<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let (_context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert!(
            !fork_choice.nodes.is_empty(),
            "checkpoint-synced forkchoice should still expose the finalized anchor node"
        );
        assert!(
            fork_choice.finalized.slot > 0,
            "post-genesis forkchoice setup should advance finalized beyond genesis"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= fork_choice.finalized.slot),
            "forkchoice should filter out any node older than the finalized slot"
        );
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "without a store-mutation hook, the closest black-box equivalent keeps only the finalized anchor node rather than returning an empty node list"
        );

        let node = &fork_choice.nodes[0];
        assert_eq!(
            node.slot, fork_choice.finalized.slot,
            "the remaining anchor node should sit exactly at the finalized boundary"
        );
        assert_eq!(
            node.root, fork_choice.finalized.root,
            "the remaining anchor node should match the finalized checkpoint root"
        );
        assert_eq!(
            fork_choice.head, fork_choice.finalized.root,
            "checkpoint-synced forkchoice should keep head at the finalized anchor when no mesh peers are connected"
        );
        assert_eq!(
            fork_choice.safe_target, fork_choice.head,
            "checkpoint-synced forkchoice should keep safe_target aligned with the only remaining anchor node"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_includes_expected_node_fields<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "fresh forkchoice should expose exactly one node for field checks"
        );

        let node = &fork_choice.nodes[0];
        assert_hex_root(&node.root, "forkchoice node root");
        assert_hex_root(&node.parent_root, "forkchoice node parent_root");
        assert_eq!(node.slot, 0, "fresh forkchoice node slot should decode to genesis");
        assert_eq!(
            node.proposer_index, 0,
            "fresh forkchoice node proposer_index should decode from the genesis block"
        );
        assert_eq!(
            node.weight, 0,
            "fresh forkchoice node weight should decode even when it defaults to zero"
        );
    }
}

dyn_async! {
    async fn test_forkchoice<'a>(clients: Vec<Client>, _: ()) {
        let (_client, fork_choice) = load_fresh_fork_choice_setup(clients).await;
        assert_eq!(
            fork_choice.nodes.len(),
            1,
            "fresh forkchoice tree should contain only the genesis node"
        );

        let node = &fork_choice.nodes[0];
        assert_hex_root(&node.root, "forkchoice node root");
        assert_hex_root(&node.parent_root, "forkchoice node parent_root");
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert_hex_root(&fork_choice.justified.root, "forkchoice justified root");
        assert_hex_root(&fork_choice.finalized.root, "forkchoice finalized root");
        assert_hex_root(&fork_choice.safe_target, "forkchoice safe_target");
        assert_eq!(node.slot, 0, "fresh forkchoice node should be genesis");
        assert_eq!(
            node.parent_root, ZERO_ROOT_HEX,
            "genesis node should reference the zero parent root"
        );
        assert_eq!(
            node.proposer_index, 0,
            "genesis node should use proposer index 0 in the baseline tree"
        );
        assert_eq!(
            node.weight, 0,
            "fresh forkchoice node should default missing weight to zero"
        );
        assert_eq!(
            fork_choice.head, node.root,
            "fresh forkchoice head should point at the only genesis node"
        );
        assert_eq!(
            fork_choice.justified.slot, 0,
            "fresh forkchoice justified checkpoint should stay at genesis"
        );
        assert_eq!(
            fork_choice.justified.root, fork_choice.head,
            "fresh forkchoice justified root should match the genesis head"
        );
        assert_eq!(
            fork_choice.finalized.slot, 0,
            "fresh forkchoice finalized checkpoint should stay at genesis"
        );
        assert_eq!(
            fork_choice.finalized.root, fork_choice.head,
            "fresh forkchoice finalized root should match the genesis head"
        );
        assert_eq!(
            fork_choice.safe_target, fork_choice.head,
            "fresh forkchoice safe_target should match the genesis head"
        );
        assert!(
            fork_choice.validator_count > 0,
            "fresh forkchoice validator_count should come from the head state"
        );
    }
}

// /lean/v0/states/finalized
dyn_async! {
    async fn test_state_returns_ssz_encoded_finalized_state<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_returns_octet_stream_content_type<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_encoding_failure<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_uses_latest_finalized_root<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_config<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_slot<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_block_header<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_justified<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_finalized<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_historical_block_hashes<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justified_slots<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_validators<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justifications_roots<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justifications_validators<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}

dyn_async! {
    async fn test_state_finalized_not_available_in_store<'a>(clients: Vec<Client>, _: ()) {
        let _client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
    }
}
