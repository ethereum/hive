use std::time::Duration;

use crate::scenarios::sync::{
    default_genesis_time, is_lean_spec_client_name, lean_spec_source_environment,
    start_post_genesis_sync_context, PostGenesisSyncContext, PostGenesisSyncTestData,
    SourceCheckpointKind,
};
use crate::{
    get_json_with_retry, lean_api_url, lean_clients, lean_environment, selected_lean_devnet_label,
    CheckpointResponse, HealthResponse, HEALTHY_STATUS, LEAN_RPC_SERVICE,
};
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use reqwest::{
    header::{ACCEPT, CONTENT_TYPE},
    Client as HttpClient,
};
use serde::Deserialize;
use ssz::Decode as SszDecode;
use ssz_derive::Decode;
use ssz_types::{
    typenum::{U1073741824, U262144, U32, U4096, U52},
    BitList, FixedVector, VariableList,
};
use tokio::time::sleep;

const FORK_CHOICE_TIMEOUT_SECS: u64 = 600;
const ZERO_ROOT_HEX: &str = "0x0000000000000000000000000000000000000000000000000000000000000000";
const SSZ_CONTENT_TYPE: &str = "application/octet-stream";

type RootBytes = FixedVector<u8, U32>;
type LeanPublicKey = FixedVector<u8, U52>;

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanConfig {
    genesis_time: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanBlockHeader {
    slot: u64,
    proposer_index: u64,
    parent_root: RootBytes,
    state_root: RootBytes,
    body_root: RootBytes,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanCheckpoint {
    root: RootBytes,
    slot: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanValidatorDevnet3 {
    public_key: LeanPublicKey,
    index: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanValidatorDevnet4 {
    attestation_public_key: LeanPublicKey,
    proposal_public_key: LeanPublicKey,
    index: u64,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct LeanValidator {
    attestation_public_key: LeanPublicKey,
    proposal_public_key: Option<LeanPublicKey>,
    index: u64,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanStateDevnet3 {
    config: LeanConfig,
    slot: u64,
    latest_block_header: LeanBlockHeader,
    latest_justified: LeanCheckpoint,
    latest_finalized: LeanCheckpoint,
    historical_block_hashes: VariableList<RootBytes, U262144>,
    justified_slots: BitList<U262144>,
    validators: VariableList<LeanValidatorDevnet3, U4096>,
    justifications_roots: VariableList<RootBytes, U262144>,
    justifications_validators: BitList<U1073741824>,
}

#[derive(Debug, Clone, PartialEq, Eq, Decode)]
struct LeanStateDevnet4 {
    config: LeanConfig,
    slot: u64,
    latest_block_header: LeanBlockHeader,
    latest_justified: LeanCheckpoint,
    latest_finalized: LeanCheckpoint,
    historical_block_hashes: VariableList<RootBytes, U262144>,
    justified_slots: BitList<U262144>,
    validators: VariableList<LeanValidatorDevnet4, U4096>,
    justifications_roots: VariableList<RootBytes, U262144>,
    justifications_validators: BitList<U1073741824>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
struct LeanState {
    config: LeanConfig,
    slot: u64,
    latest_block_header: LeanBlockHeader,
    latest_justified: LeanCheckpoint,
    latest_finalized: LeanCheckpoint,
    historical_block_hashes: VariableList<RootBytes, U262144>,
    justified_slots: BitList<U262144>,
    validators: Vec<LeanValidator>,
    justifications_roots: VariableList<RootBytes, U262144>,
    justifications_validators: BitList<U1073741824>,
}

impl From<LeanStateDevnet3> for LeanState {
    fn from(state: LeanStateDevnet3) -> Self {
        Self {
            config: state.config,
            slot: state.slot,
            latest_block_header: state.latest_block_header,
            latest_justified: state.latest_justified,
            latest_finalized: state.latest_finalized,
            historical_block_hashes: state.historical_block_hashes,
            justified_slots: state.justified_slots,
            validators: state
                .validators
                .into_iter()
                .map(|validator| LeanValidator {
                    attestation_public_key: validator.public_key,
                    proposal_public_key: None,
                    index: validator.index,
                })
                .collect(),
            justifications_roots: state.justifications_roots,
            justifications_validators: state.justifications_validators,
        }
    }
}

impl From<LeanStateDevnet4> for LeanState {
    fn from(state: LeanStateDevnet4) -> Self {
        Self {
            config: state.config,
            slot: state.slot,
            latest_block_header: state.latest_block_header,
            latest_justified: state.latest_justified,
            latest_finalized: state.latest_finalized,
            historical_block_hashes: state.historical_block_hashes,
            justified_slots: state.justified_slots,
            validators: state
                .validators
                .into_iter()
                .map(|validator| LeanValidator {
                    attestation_public_key: validator.attestation_public_key,
                    proposal_public_key: Some(validator.proposal_public_key),
                    index: validator.index,
                })
                .collect(),
            justifications_roots: state.justifications_roots,
            justifications_validators: state.justifications_validators,
        }
    }
}

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
    let mut stalled_post_genesis_attempts = 0;

    for _attempt in 0..FORK_CHOICE_TIMEOUT_SECS {
        let fork_choice = load_fork_choice_response(client).await;
        let checkpoint_slot = match checkpoint_kind {
            SourceCheckpointKind::Justified => fork_choice.justified.slot,
            SourceCheckpointKind::Finalized => fork_choice.finalized.slot,
        };
        if checkpoint_slot > 0 {
            return fork_choice;
        }

        if matches!(checkpoint_kind, SourceCheckpointKind::Finalized) {
            let has_post_genesis_progress = fork_choice.justified.slot > 0
                || fork_choice.nodes.iter().any(|node| node.slot > 0);

            if has_post_genesis_progress {
                stalled_post_genesis_attempts += 1;
                if stalled_post_genesis_attempts >= 30 {
                    panic!(
                        "Client {} advanced post-genesis but never reported a non-genesis finalized forkchoice checkpoint (justified slot: {}, max node slot: {})",
                        client.kind,
                        fork_choice.justified.slot,
                        fork_choice.nodes.iter().map(|node| node.slot).max().unwrap_or(0)
                    );
                }
            } else {
                stalled_post_genesis_attempts = 0;
            }
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

async fn wait_for_post_genesis_fork_choice_response(client: &Client) -> ForkChoiceResponse {
    for _attempt in 0..FORK_CHOICE_TIMEOUT_SECS {
        let fork_choice = load_fork_choice_response(client).await;
        if fork_choice.finalized.slot > 0
            || fork_choice.justified.slot > 0
            || fork_choice.nodes.iter().any(|node| node.slot > 0)
        {
            return fork_choice;
        }

        sleep(Duration::from_secs(1)).await;
    }

    panic!(
        "Client {} never exposed a post-genesis forkchoice response",
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
    let fork_choice = match checkpoint_kind {
        SourceCheckpointKind::Justified => {
            wait_for_non_genesis_fork_choice_response(&context.client_under_test, checkpoint_kind)
                .await
        }
        SourceCheckpointKind::Finalized => {
            wait_for_post_genesis_fork_choice_response(&context.client_under_test).await
        }
    };
    (context, fork_choice)
}

async fn load_response_with_retry(
    client: &Client,
    path: &str,
    accept_content_type: Option<&str>,
) -> reqwest::Response {
    let http = http_client();
    let url = lean_api_url(client, path);
    let mut last_error = String::new();

    for _attempt in 0..10 {
        let mut request = http.get(&url);
        if let Some(accept_content_type) = accept_content_type {
            request = request.header(ACCEPT, accept_content_type);
        }

        match request.send().await {
            Ok(response) => {
                let status = response.status();
                if status.is_success() {
                    return response;
                }

                last_error = format!("received HTTP {status} from {url}");
            }
            Err(err) => {
                last_error = format!("error sending request for url ({url}): {err}");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    panic!("Request to {url} did not succeed after retries: {last_error}");
}

async fn load_finalized_state_response(client: &Client) -> reqwest::Response {
    load_response_with_retry(client, "/lean/v0/states/finalized", Some(SSZ_CONTENT_TYPE)).await
}

async fn load_finalized_state_bytes(client: &Client) -> Vec<u8> {
    let response = load_finalized_state_response(client).await;
    response
        .bytes()
        .await
        .unwrap_or_else(|err| panic!("Unable to read finalized state response body: {err}"))
        .to_vec()
}

fn decode_finalized_state(ssz_bytes: &[u8]) -> LeanState {
    if selected_lean_devnet_label() == "devnet4" {
        LeanStateDevnet4::from_ssz_bytes(ssz_bytes)
            .map(Into::into)
            .unwrap_or_else(|err| panic!("Unable to decode SSZ finalized state: {err:?}"))
    } else {
        LeanStateDevnet3::from_ssz_bytes(ssz_bytes)
            .map(Into::into)
            .unwrap_or_else(|err| panic!("Unable to decode SSZ finalized state: {err:?}"))
    }
}

async fn load_finalized_state(client: &Client) -> LeanState {
    decode_finalized_state(&load_finalized_state_bytes(client).await)
}

async fn load_fresh_state_setup(clients: Vec<Client>) -> (Client, LeanState) {
    let client = expect_single_client(clients);
    let state = load_finalized_state(&client).await;
    (client, state)
}

async fn load_fresh_state_and_fork_choice_setup(
    clients: Vec<Client>,
) -> (Client, LeanState, ForkChoiceResponse) {
    let (client, state) = load_fresh_state_setup(clients).await;
    let fork_choice = load_fork_choice_response(&client).await;
    (client, state, fork_choice)
}

async fn load_post_genesis_state_setup(
    clients: Vec<Client>,
    test_data: PostGenesisSyncTestData,
) -> (PostGenesisSyncContext, LeanState, ForkChoiceResponse) {
    let context = load_post_genesis_sync_context(clients, test_data).await;
    let state = load_finalized_state(&context.client_under_test).await;
    let fork_choice = if selected_lean_devnet_label() == "devnet4" {
        load_fork_choice_response(&context.client_under_test).await
    } else {
        wait_for_non_genesis_fork_choice_response(
            &context.client_under_test,
            SourceCheckpointKind::Finalized,
        )
        .await
    };
    (context, state, fork_choice)
}

dyn_async! {
    pub async fn run_rpc_compat_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let available_clients = test.sim.client_types().await;
        let lean_spec_client = available_clients
            .iter()
            .find(|client| is_lean_spec_client_name(&client.name))
            .cloned()
            .expect("The `lean-spec-client` helper client should always be available for the Lean simulator");
        let clients: Vec<_> = lean_clients(available_clients)
            .into_iter()
            .filter(|client| !is_lean_spec_client_name(&client.name))
            .collect();
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in &clients {
            test.run(NClientTestSpec {
                name: "health healthy".to_string(),
                description: "rpc_compat: Checks that the health endpoint reports a healthy Lean RPC service."
                    .to_string(),
                always_run: false,
                run: test_health_healthy,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: checkpoints justified root encoding".to_string(),
                description: "Checks that the justified checkpoint root is hex encoded.".to_string(),
                always_run: false,
                run: test_checkpoints_hex_encodes_root,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: checkpoints justified fields".to_string(),
                description:
                    "Checks that the justified checkpoint endpoint returns the expected fields."
                        .to_string(),
                always_run: false,
                run: test_checkpoints_returns_expected_fields,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: checkpoints justified genesis".to_string(),
                description: "Checks that a fresh Lean node reports the genesis justified checkpoint."
                    .to_string(),
                always_run: false,
                run: test_checkpoints_returns_genesis_justified_checkpoint_for_fresh_node,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            let checkpoint_genesis_time = default_genesis_time();

            test.run(NClientTestSpec {
                name: "rpc_compat: checkpoints justified post-genesis".to_string(),
                description: "Waits for the LeanSpec source to finalize, checkpoint-syncs the client under test from that source, and checks that the client under test reaches a non-genesis justified checkpoint.".to_string(),
                always_run: false,
                run: test_checkpoints_justified,
                environments: Some(vec![Some(lean_spec_source_environment(
                    checkpoint_genesis_time,
                ))]),
                test_data: PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: checkpoint_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: true,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: false,
                },
                clients: vec![lean_spec_client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice no head".to_string(),
                description:
                    "Loads the forkchoice endpoint from a fresh node before any non-genesis head advancement."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_no_head,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice no justified".to_string(),
                description:
                    "Loads the forkchoice endpoint from a fresh node before any non-genesis justified checkpoint exists."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_no_justified,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice no finalized".to_string(),
                description:
                    "Loads the forkchoice endpoint from a fresh node before any non-genesis finalized checkpoint exists."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_no_finalized,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice no nodes".to_string(),
                description:
                    "Loads the forkchoice endpoint from a fresh node before any non-genesis nodes are present."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_no_nodes,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice defaults missing weight to zero".to_string(),
                description:
                    "Loads the forkchoice endpoint from a fresh node where block weights should still be zero."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_defaults_missing_weight_to_zero,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice zero validator count when head state missing".to_string(),
                description:
                    "Loads the forkchoice endpoint from the closest black-box baseline available before a missing head-state hook exists."
                        .to_string(),
                always_run: false,
                run: test_forkchoice_zero_validator_count_when_head_state_missing,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice hex encodes roots".to_string(),
                description: "Loads the forkchoice endpoint to prepare root encoding assertions."
                    .to_string(),
                always_run: false,
                run: test_forkchoice_hex_encodes_roots,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice includes expected node fields".to_string(),
                description: "Loads the forkchoice endpoint to prepare node field assertions."
                    .to_string(),
                always_run: false,
                run: test_forkchoice_includes_expected_node_fields,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice".to_string(),
                description: "Loads the forkchoice endpoint for the baseline RPC compatibility case."
                    .to_string(),
                always_run: false,
                run: test_forkchoice,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            let finalized_filters_genesis_time = default_genesis_time();

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice filters nodes before finalized slot".to_string(),
                description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and loads forkchoice with a non-genesis finalized slot.".to_string(),
                always_run: false,
                run: test_forkchoice_filters_nodes_before_finalized_slot,
                environments: Some(vec![Some(lean_spec_source_environment(
                    finalized_filters_genesis_time,
                ))]),
                test_data: PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: finalized_filters_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: false,
                },
                clients: vec![lean_spec_client.clone()],
            }).await;

            let finalized_boundary_genesis_time = default_genesis_time();

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice keeps nodes at or beyond finalized slot".to_string(),
                description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and checks that the visible forkchoice nodes stay at or beyond the finalized boundary.".to_string(),
                always_run: false,
                run: test_forkchoice_keeps_nodes_at_or_beyond_finalized_slot,
                environments: Some(vec![Some(lean_spec_source_environment(
                    finalized_boundary_genesis_time,
                ))]),
                test_data: PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: finalized_boundary_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: false,
                },
                clients: vec![lean_spec_client.clone()],
            }).await;

            let pre_finalized_only_genesis_time = default_genesis_time();

            test.run(NClientTestSpec {
                name: "rpc_compat: forkchoice returns empty nodes when all blocks are pre-finalized".to_string(),
                description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and loads forkchoice at the finalized boundary.".to_string(),
                always_run: false,
                run: test_forkchoice_returns_empty_nodes_when_all_blocks_are_pre_finalized,
                environments: Some(vec![Some(lean_spec_source_environment(
                    pre_finalized_only_genesis_time,
                ))]),
                test_data: PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: pre_finalized_only_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: false,
                },
                clients: vec![lean_spec_client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state returns ssz encoded finalized state".to_string(),
                description: "Loads the finalized state endpoint and checks that it returns decodable SSZ bytes."
                    .to_string(),
                always_run: false,
                run: test_state_returns_ssz_encoded_finalized_state,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state returns octet-stream content type".to_string(),
                description: "Loads the finalized state endpoint and checks that it responds with application/octet-stream."
                    .to_string(),
                always_run: false,
                run: test_state_returns_octet_stream_content_type,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes config".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the config field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_config,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes slot".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the slot field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_slot,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes latest block header".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the latest_block_header field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_latest_block_header,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes latest justified".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the latest_justified checkpoint."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_latest_justified,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes latest finalized".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the latest_finalized checkpoint."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_latest_finalized,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes historical block hashes".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the historical_block_hashes field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_historical_block_hashes,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes justified slots".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the justified_slots field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_justified_slots,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes validators".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the validators field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_validators,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes justifications roots".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the justifications_roots field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_justifications_roots,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state ssz decodes justifications validators".to_string(),
                description: "Loads and SSZ-decodes the finalized state, then checks the justifications_validators field."
                    .to_string(),
                always_run: false,
                run: test_state_ssz_decodes_justifications_validators,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            test.run(NClientTestSpec {
                name: "rpc_compat: state decodes".to_string(),
                description: "Loads the finalized state endpoint for the baseline RPC compatibility case."
                    .to_string(),
                always_run: false,
                run: test_state,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client.clone()],
            }).await;

            let state_finalized_genesis_time = default_genesis_time();

            test.run(NClientTestSpec {
                name: "rpc_compat: state finalized endpoint tracks latest finalized slot".to_string(),
                description: "Starts a LeanSpec client mesh, checkpoint-syncs the client under test to a finalized checkpoint, and checks that the finalized state endpoint tracks the client's latest finalized slot."
                    .to_string(),
                always_run: false,
                run: test_state_finalized_endpoint_tracks_latest_finalized_slot,
                environments: Some(vec![Some(lean_spec_source_environment(
                    state_finalized_genesis_time,
                ))]),
                test_data: PostGenesisSyncTestData {
                    client_under_test: client.clone(),
                    genesis_time: state_finalized_genesis_time,
                    source_checkpoint_kind: SourceCheckpointKind::Finalized,
                    wait_for_client_justified_checkpoint: false,
                    use_checkpoint_sync: true,
                    connect_client_to_lean_spec_mesh: false,
                },
                clients: vec![lean_spec_client.clone()],
            }).await;
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
        let _ = (&context.source_client, &context.client_under_test);
        let client_checkpoint = context
            .client_checkpoint
            .as_ref()
            .expect("checkpoint tests should wait for a client justified checkpoint");

        assert!(
            context.source_fork_choice.justified.slot > 0,
            "helper source should reach a non-genesis justified checkpoint before syncing the client under test"
        );
        assert!(
            client_checkpoint.slot > 0,
            "client under test should report a non-genesis justified checkpoint after the helper mesh reaches justification"
        );

        if client_checkpoint.slot == context.source_fork_choice.justified.slot {
            assert_eq!(
                client_checkpoint.root,
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
        let (context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        let reference_finalized_slot = context.source_fork_choice.finalized.slot;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert_hex_root(&fork_choice.finalized.root, "forkchoice finalized root");
        assert!(
            !fork_choice.nodes.is_empty(),
            "forkchoice should still expose the finalized boundary node"
        );
        assert!(
            reference_finalized_slot > 0,
            "helper-backed forkchoice setup should sync from a non-genesis finalized boundary"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= reference_finalized_slot),
            "forkchoice should filter out any node older than the checkpoint-sync finalized boundary"
        );
    }
}

dyn_async! {
    async fn test_forkchoice_keeps_nodes_at_or_beyond_finalized_slot<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let (context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        let reference_finalized_slot = context.source_fork_choice.finalized.slot;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert_hex_root(&fork_choice.finalized.root, "forkchoice finalized root");
        assert!(
            !fork_choice.nodes.is_empty(),
            "forkchoice should still expose the finalized boundary node"
        );
        assert!(
            reference_finalized_slot > 0,
            "helper-backed forkchoice setup should sync from a non-genesis finalized boundary"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= reference_finalized_slot),
            "forkchoice should keep visible nodes at or beyond the checkpoint-sync finalized boundary"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .any(|node| node.slot >= reference_finalized_slot),
            "forkchoice should keep at least one node at or beyond the finalized boundary"
        );
        assert!(
            fork_choice.justified.slot > 0,
            "checkpoint-synced forkchoice should still expose a post-genesis justified checkpoint"
        );
        assert_hex_root(&fork_choice.safe_target, "forkchoice safe_target");
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
        let (context, fork_choice) =
            load_post_genesis_fork_choice_setup(clients, test_data, SourceCheckpointKind::Finalized)
                .await;
        let reference_finalized_slot = context.source_fork_choice.finalized.slot;
        assert_hex_root(&fork_choice.head, "forkchoice head");
        assert!(
            reference_finalized_slot > 0,
            "helper-backed forkchoice setup should sync from a non-genesis finalized boundary"
        );
        assert!(
            fork_choice
                .nodes
                .iter()
                .all(|node| node.slot >= reference_finalized_slot),
            "forkchoice should keep only nodes at or beyond the checkpoint-sync finalized boundary"
        );
        assert!(
            fork_choice.nodes.is_empty()
                || fork_choice
                    .nodes
                    .iter()
                    .all(|node| node.root != ZERO_ROOT_HEX),
            "forkchoice should still return well-formed node roots when it exposes post-finalized nodes"
        );
        assert!(
            fork_choice.justified.slot > 0
                || !fork_choice.nodes.is_empty(),
            "checkpoint-synced forkchoice should expose either post-genesis justification or a compact post-finalized node set"
        );
        assert_hex_root(&fork_choice.safe_target, "forkchoice safe_target");
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
        let client = expect_single_client(clients);
        let ssz_bytes = load_finalized_state_bytes(&client).await;

        assert!(
            !ssz_bytes.is_empty(),
            "finalized state endpoint should return a non-empty SSZ payload"
        );

        let state = decode_finalized_state(&ssz_bytes);
        assert!(
            !state.validators.is_empty(),
            "decoded finalized state should include at least one validator"
        );
    }
}

dyn_async! {
    async fn test_state_returns_octet_stream_content_type<'a>(clients: Vec<Client>, _: ()) {
        let client = expect_single_client(clients);
        let response = load_finalized_state_response(&client).await;
        let content_type = response
            .headers()
            .get(CONTENT_TYPE)
            .and_then(|value| value.to_str().ok())
            .expect("finalized state endpoint should return a content-type header");

        assert_eq!(
            content_type, "application/octet-stream",
            "finalized state endpoint should return application/octet-stream"
        );
    }
}

dyn_async! {
    async fn test_state_finalized_endpoint_tracks_latest_finalized_slot<'a>(clients: Vec<Client>, test_data: PostGenesisSyncTestData) {
        let (_context, state, fork_choice) = load_post_genesis_state_setup(clients, test_data).await;

        assert_eq!(
            state.latest_finalized.slot, fork_choice.finalized.slot,
            "finalized state endpoint should expose the client's latest finalized slot through the embedded latest_finalized checkpoint"
        );
        assert!(
            state.slot >= state.latest_finalized.slot,
            "the returned finalized state should not be behind its embedded latest_finalized checkpoint"
        );
        assert!(
            state.latest_block_header.slot >= state.latest_finalized.slot,
            "the finalized state's latest_block_header should stay at or ahead of the embedded latest_finalized checkpoint"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_config<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            state.config.genesis_time > 0,
            "decoded finalized state config should include a non-zero genesis_time"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_slot<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state, fork_choice) = load_fresh_state_and_fork_choice_setup(clients).await;

        assert_eq!(
            state.slot, fork_choice.finalized.slot,
            "fresh finalized state should decode to the same slot reported by forkchoice.finalized"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_block_header<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert_eq!(
            state.latest_block_header.slot, state.slot,
            "finalized state should expose a latest_block_header aligned with the state slot"
        );
        assert!(
            state.latest_block_header.proposer_index < state.validators.len() as u64,
            "finalized state latest_block_header proposer_index should decode within validator range"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_justified<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state, fork_choice) = load_fresh_state_and_fork_choice_setup(clients).await;

        assert_eq!(
            state.latest_justified.slot, fork_choice.justified.slot,
            "finalized state latest_justified slot should match the justified checkpoint reported by forkchoice"
        );
        assert!(
            state.latest_justified.slot >= state.latest_finalized.slot,
            "decoded latest_justified checkpoint should stay at or ahead of latest_finalized"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_latest_finalized<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state, fork_choice) = load_fresh_state_and_fork_choice_setup(clients).await;

        assert_eq!(
            state.latest_finalized.slot, fork_choice.finalized.slot,
            "finalized state latest_finalized slot should match the finalized checkpoint reported by forkchoice"
        );
        assert!(
            state.latest_finalized.slot <= state.slot,
            "decoded latest_finalized checkpoint should not be ahead of the state slot"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_historical_block_hashes<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            state.historical_block_hashes.len() <= state.slot as usize + 1,
            "historical_block_hashes should not contain more entries than reachable finalized slots"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justified_slots<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            state.justified_slots.num_set_bits() <= state.justified_slots.len(),
            "justified_slots should decode into a well-formed bitlist"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_validators<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            !state.validators.is_empty(),
            "finalized state should decode at least one validator"
        );

        for (index, validator) in state.validators.iter().enumerate() {
            assert_eq!(
                validator.index, index as u64,
                "validator indices should decode in registry order"
            );
        }
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justifications_roots<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            state.justifications_roots.len() <= state.historical_block_hashes.len(),
            "justifications_roots should only reference tracked historical roots"
        );
    }
}

dyn_async! {
    async fn test_state_ssz_decodes_justifications_validators<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state) = load_fresh_state_setup(clients).await;

        assert!(
            state.justifications_validators.num_set_bits() <= state.justifications_validators.len(),
            "justifications_validators should decode into a well-formed bitlist"
        );
    }
}

dyn_async! {
    async fn test_state<'a>(clients: Vec<Client>, _: ()) {
        let (_client, state, fork_choice) = load_fresh_state_and_fork_choice_setup(clients).await;

        assert!(
            !state.validators.is_empty(),
            "finalized state should decode a non-empty validator registry"
        );
        assert_eq!(
            state.latest_justified.slot, fork_choice.justified.slot,
            "finalized state should align its latest_justified slot with forkchoice"
        );
        assert_eq!(
            state.latest_finalized.slot, fork_choice.finalized.slot,
            "finalized state should align its latest_finalized slot with forkchoice"
        );
        assert!(
            state.slot >= state.latest_finalized.slot,
            "finalized state slot should stay at or ahead of its latest finalized checkpoint"
        );
    }
}
