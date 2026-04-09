use std::collections::HashMap;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::{get_json_with_retry, lean_api_url, lean_environment, CheckpointResponse};
use hivesim::types::ClientDefinition;
use hivesim::Client;
use reqwest::Client as HttpClient;
use serde::Deserialize;
use tokio::time::sleep;

pub(crate) const LEAN_SPEC_CLIENT_TYPE: &str = "lean-spec-client";

const CHECKPOINT_SYNC_URL_ENVIRONMENT_VARIABLE: &str = "HIVE_CHECKPOINT_SYNC_URL";
const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const LEAN_SPEC_SOURCE_NODE_ID: &str = "lean_spec_0";
const LEAN_SPEC_PEER_NODE_ID: &str = "lean_spec_1";
const LEAN_SPEC_SOURCE_VALIDATORS: &str = "0,1";
const LEAN_SPEC_PEER_VALIDATORS: &str = "2";
const LEAN_P2P_PORT: u16 = 9001;
const LEAN_GENESIS_DELAY_SECS: u64 = 30;
const POST_GENESIS_TIMEOUT_SECS: u64 = 180;

#[allow(dead_code)]
#[derive(Clone, Copy)]
pub(crate) enum SourceCheckpointKind {
    Justified,
    Finalized,
}

#[derive(Clone)]
pub(crate) struct PostGenesisSyncTestData {
    pub client_under_test: ClientDefinition,
    pub genesis_time: u64,
    pub source_checkpoint_kind: SourceCheckpointKind,
    pub use_checkpoint_sync: bool,
    pub connect_client_to_lean_spec_mesh: bool,
}

pub(crate) struct PostGenesisSyncContext {
    pub source_client: Client,
    pub peer_client: Client,
    pub client_under_test: Client,
    pub source_fork_choice: ForkChoiceSnapshot,
    pub client_checkpoint: CheckpointResponse,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ForkChoiceSnapshot {
    pub justified: CheckpointResponse,
    pub finalized: CheckpointResponse,
}

pub(crate) fn default_genesis_time() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time is before UNIX_EPOCH")
        .as_secs()
        + LEAN_GENESIS_DELAY_SECS
}

pub(crate) fn lean_spec_source_environment(genesis_time: u64) -> HashMap<String, String> {
    lean_spec_environment(
        LEAN_SPEC_SOURCE_NODE_ID,
        LEAN_SPEC_SOURCE_VALIDATORS,
        genesis_time,
        None,
        true,
    )
}

pub(crate) fn lean_spec_peer_environment(
    source_client: &Client,
    genesis_time: u64,
) -> HashMap<String, String> {
    lean_spec_environment(
        LEAN_SPEC_PEER_NODE_ID,
        LEAN_SPEC_PEER_VALIDATORS,
        genesis_time,
        Some(bootnode_multiaddr(source_client)),
        false,
    )
}

pub(crate) async fn start_post_genesis_sync_context(
    source_client: Client,
    test_data: &PostGenesisSyncTestData,
) -> PostGenesisSyncContext {
    let peer_client = source_client
        .test
        .start_client(
            source_client.kind.clone(),
            Some(lean_spec_peer_environment(&source_client, test_data.genesis_time)),
        )
        .await;

    let source_fork_choice =
        wait_for_non_genesis_fork_choice(&source_client, test_data.source_checkpoint_kind).await;

    let client_under_test = source_client
        .test
        .start_client(
            test_data.client_under_test.name.clone(),
            Some(client_under_test_environment(
                &source_client,
                test_data.use_checkpoint_sync,
                test_data.connect_client_to_lean_spec_mesh,
            )),
        )
        .await;

    let client_checkpoint = wait_for_non_genesis_justified_checkpoint(&client_under_test).await;

    PostGenesisSyncContext {
        source_client,
        peer_client,
        client_under_test,
        source_fork_choice,
        client_checkpoint,
    }
}

fn lean_spec_environment(
    node_id: &str,
    validator_indices: &str,
    genesis_time: u64,
    bootnodes: Option<String>,
    is_aggregator: bool,
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    environment.insert(
        LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE.to_string(),
        genesis_time.to_string(),
    );
    environment.insert(NODE_ID_ENVIRONMENT_VARIABLE.to_string(), node_id.to_string());

    if !validator_indices.is_empty() {
        environment.insert(
            LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE.to_string(),
            validator_indices.to_string(),
        );
    }

    if let Some(bootnodes) = bootnodes {
        environment.insert(BOOTNODES_ENVIRONMENT_VARIABLE.to_string(), bootnodes);
    }

    if is_aggregator {
        environment.insert(
            IS_AGGREGATOR_ENVIRONMENT_VARIABLE.to_string(),
            "1".to_string(),
        );
    }

    environment
}

fn client_under_test_environment(
    checkpoint_source: &Client,
    use_checkpoint_sync: bool,
    connect_to_lean_spec_mesh: bool,
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    if use_checkpoint_sync {
        environment.insert(
            CHECKPOINT_SYNC_URL_ENVIRONMENT_VARIABLE.to_string(),
            checkpoint_sync_url(checkpoint_source),
        );
    }

    if connect_to_lean_spec_mesh {
        environment.insert(
            BOOTNODES_ENVIRONMENT_VARIABLE.to_string(),
            bootnode_multiaddr(checkpoint_source),
        );
    }

    environment
}

fn checkpoint_sync_url(client: &Client) -> String {
    format!("http://{}:5052", client.ip)
}

fn bootnode_multiaddr(client: &Client) -> String {
    format!("/ip4/{}/udp/{}/quic-v1", client.ip, LEAN_P2P_PORT)
}

async fn wait_for_non_genesis_fork_choice(
    client: &Client,
    checkpoint_kind: SourceCheckpointKind,
) -> ForkChoiceSnapshot {
    let http = http_client();
    let checkpoint_name = match checkpoint_kind {
        SourceCheckpointKind::Justified => "justified",
        SourceCheckpointKind::Finalized => "finalized",
    };

    for _attempt in 0..POST_GENESIS_TIMEOUT_SECS {
        let fork_choice: ForkChoiceSnapshot =
            get_json_with_retry(&http, &lean_api_url(client, "/lean/v0/fork_choice")).await;
        let checkpoint = match checkpoint_kind {
            SourceCheckpointKind::Justified => &fork_choice.justified,
            SourceCheckpointKind::Finalized => &fork_choice.finalized,
        };
        if checkpoint.slot > 0 {
            return fork_choice;
        }

        sleep(Duration::from_secs(1)).await;
    }

    panic!(
        "LeanSpec source client {} never reached a non-genesis {checkpoint_name} checkpoint",
        client.kind,
    );
}

async fn wait_for_non_genesis_justified_checkpoint(client: &Client) -> CheckpointResponse {
    let http = http_client();

    for _attempt in 0..POST_GENESIS_TIMEOUT_SECS {
        let checkpoint: CheckpointResponse =
            get_json_with_retry(&http, &lean_api_url(client, "/lean/v0/checkpoints/justified"))
                .await;
        if checkpoint.slot > 0 {
            return checkpoint;
        }

        sleep(Duration::from_secs(1)).await;
    }

    panic!(
        "Client under test {} never reached a non-genesis justified checkpoint",
        client.kind
    );
}

fn http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("Unable to build HTTP client")
}
