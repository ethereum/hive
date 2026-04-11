use std::collections::HashMap;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::{lean_api_url, lean_environment, CheckpointResponse};
use hivesim::types::ClientDefinition;
use hivesim::Client;
use reqwest::Client as HttpClient;
use serde::Deserialize;
use tokio::time::sleep;

pub(crate) const LEAN_SPEC_CLIENT_TYPE: &str = "lean-spec-client";

const CHECKPOINT_SYNC_URL_ENVIRONMENT_VARIABLE: &str = "HIVE_CHECKPOINT_SYNC_URL";
const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const LEAN_GENESIS_VALIDATORS_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_VALIDATORS";
const LEAN_GENESIS_VALIDATOR_ENTRIES_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const LEAN_SPEC_SOURCE_NODE_ID: &str = "lean_spec_0";
const LEAN_SPEC_SOURCE_VALIDATORS: &str = "0,1,2";
const LEAN_SPEC_SOURCE_PEER_ID: &str = "16Uiu2HAmHzBkRq62mG95vsjKMuYQBezZCtjPXYWUoyVxMxi71aB3";
const LEAN_P2P_PORT: u16 = 9001;
const LEAN_GENESIS_DELAY_SECS: u64 = 30;
const POST_GENESIS_TIMEOUT_SECS: u64 = 600;
const MIN_FINALIZED_SLOT_FOR_CHECKPOINT_SYNC: u64 = 10;
const HELPER_METADATA_PORT: u16 = 5053;

#[derive(Debug, Deserialize)]
struct HelperGenesisValidatorEntry {
    attestation_public_key: String,
    proposal_public_key: Option<String>,
}

#[derive(Debug, Deserialize)]
struct HelperGenesisMetadata {
    genesis_validator_entries: Vec<HelperGenesisValidatorEntry>,
}

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
    pub wait_for_client_justified_checkpoint: bool,
    pub use_checkpoint_sync: bool,
    pub connect_client_to_lean_spec_mesh: bool,
}

pub(crate) struct PostGenesisSyncContext {
    pub source_client: Client,
    pub client_under_test: Client,
    pub source_fork_choice: ForkChoiceSnapshot,
    pub client_checkpoint: Option<CheckpointResponse>,
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

pub(crate) async fn start_post_genesis_sync_context(
    source_client: Client,
    test_data: &PostGenesisSyncTestData,
) -> PostGenesisSyncContext {
    let source_genesis_validator_entries = load_finalized_genesis_validators(&source_client)
        .await
        .unwrap_or_else(|err| {
            panic!(
                "Unable to load finalized genesis validators from {}: {err}",
                source_client.kind
            )
        });
    let should_start_client_early =
        !test_data.use_checkpoint_sync && test_data.connect_client_to_lean_spec_mesh;

    let client_under_test = if should_start_client_early {
        Some(
            source_client
                .test
                .start_client(
                    test_data.client_under_test.name.clone(),
                    Some(client_under_test_environment(
                        &source_client,
                        test_data.genesis_time,
                        &source_genesis_validator_entries,
                        test_data.use_checkpoint_sync,
                        test_data.connect_client_to_lean_spec_mesh,
                    )),
                )
                .await,
        )
    } else {
        None
    };

    let source_fork_choice = match wait_for_checkpoint_slot(
        &source_client,
        test_data.source_checkpoint_kind,
        minimum_source_checkpoint_slot(test_data),
    )
    .await
    {
        Ok(source_fork_choice) => source_fork_choice,
        Err(err) => {
            if client_under_test.is_none() {
                let _ = source_client
                    .test
                    .start_client(
                        test_data.client_under_test.name.clone(),
                        Some(client_under_test_environment(
                            &source_client,
                            test_data.genesis_time,
                            &source_genesis_validator_entries,
                            test_data.use_checkpoint_sync,
                            test_data.connect_client_to_lean_spec_mesh,
                        )),
                    )
                    .await;
            }

            panic!("{err}");
        }
    };

    let client_under_test = match client_under_test {
        Some(client_under_test) => client_under_test,
        None => {
            source_client
                .test
                .start_client(
                    test_data.client_under_test.name.clone(),
                    Some(client_under_test_environment(
                        &source_client,
                        test_data.genesis_time,
                        &source_genesis_validator_entries,
                        test_data.use_checkpoint_sync,
                        test_data.connect_client_to_lean_spec_mesh,
                    )),
                )
                .await
        }
    };

    let client_checkpoint = if test_data.wait_for_client_justified_checkpoint {
        Some(
            wait_for_non_genesis_justified_checkpoint(&client_under_test)
                .await
                .unwrap_or_else(|err| panic!("{err}")),
        )
    } else {
        None
    };

    PostGenesisSyncContext {
        source_client,
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
    environment.insert(
        NODE_ID_ENVIRONMENT_VARIABLE.to_string(),
        node_id.to_string(),
    );

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
    genesis_time: u64,
    source_genesis_validator_entries: &[HelperGenesisValidatorEntry],
    use_checkpoint_sync: bool,
    connect_to_lean_spec_mesh: bool,
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    environment.insert(
        LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE.to_string(),
        genesis_time.to_string(),
    );
    if source_genesis_validator_entries
        .iter()
        .all(|entry| entry.proposal_public_key.is_some())
    {
        environment.insert(
            LEAN_GENESIS_VALIDATOR_ENTRIES_ENVIRONMENT_VARIABLE.to_string(),
            source_genesis_validator_entries
                .iter()
                .map(|entry| {
                    format!(
                        "{}|{}",
                        entry.attestation_public_key,
                        entry
                            .proposal_public_key
                            .as_ref()
                            .expect("proposal key should exist for structured entries")
                    )
                })
                .collect::<Vec<_>>()
                .join(","),
        );
    } else {
        environment.insert(
            LEAN_GENESIS_VALIDATORS_ENVIRONMENT_VARIABLE.to_string(),
            source_genesis_validator_entries
                .iter()
                .map(|entry| entry.attestation_public_key.clone())
                .collect::<Vec<_>>()
                .join(","),
        );
    }
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
    format!(
        "/ip4/{}/udp/{}/quic-v1/p2p/{}",
        client.ip, LEAN_P2P_PORT, LEAN_SPEC_SOURCE_PEER_ID
    )
}

fn minimum_source_checkpoint_slot(test_data: &PostGenesisSyncTestData) -> u64 {
    if test_data.use_checkpoint_sync {
        MIN_FINALIZED_SLOT_FOR_CHECKPOINT_SYNC
    } else {
        1
    }
}

async fn wait_for_checkpoint_slot(
    client: &Client,
    checkpoint_kind: SourceCheckpointKind,
    minimum_slot: u64,
) -> Result<ForkChoiceSnapshot, String> {
    let http = http_client();
    let url = lean_api_url(client, "/lean/v0/fork_choice");
    let checkpoint_name = match checkpoint_kind {
        SourceCheckpointKind::Justified => "justified",
        SourceCheckpointKind::Finalized => "finalized",
    };
    let mut last_error = String::new();
    let mut last_observed_slot = None;

    for _attempt in 0..POST_GENESIS_TIMEOUT_SECS {
        match http.get(&url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    match response.json::<ForkChoiceSnapshot>().await {
                        Ok(fork_choice) => {
                            let checkpoint = match checkpoint_kind {
                                SourceCheckpointKind::Justified => &fork_choice.justified,
                                SourceCheckpointKind::Finalized => &fork_choice.finalized,
                            };
                            last_observed_slot = Some(checkpoint.slot);
                            if checkpoint.slot >= minimum_slot {
                                return Ok(fork_choice);
                            }
                        }
                        Err(err) => {
                            last_error =
                                format!("Unable to decode fork_choice response from {url}: {err}");
                        }
                    }
                }
            }
            Err(err) => {
                last_error = format!("error sending request for url ({url}): {err}");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "LeanSpec source client {} never reached {checkpoint_name} slot {} (last observed slot: {}, last error: {})",
        client.kind,
        minimum_slot,
        last_observed_slot
            .map(|slot| slot.to_string())
            .unwrap_or_else(|| "none".to_string()),
        if last_error.is_empty() {
            "none".to_string()
        } else {
            last_error
        }
    ))
}

async fn wait_for_non_genesis_justified_checkpoint(
    client: &Client,
) -> Result<CheckpointResponse, String> {
    let http = http_client();
    let url = lean_api_url(client, "/lean/v0/checkpoints/justified");
    let mut last_error = String::new();
    let mut last_observed_slot = None;

    for _attempt in 0..POST_GENESIS_TIMEOUT_SECS {
        match http.get(&url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    match response.json::<CheckpointResponse>().await {
                        Ok(checkpoint) => {
                            last_observed_slot = Some(checkpoint.slot);
                            if checkpoint.slot > 0 {
                                return Ok(checkpoint);
                            }
                        }
                        Err(err) => {
                            last_error = format!(
                                "Unable to decode justified checkpoint response from {url}: {err}"
                            );
                        }
                    }
                }
            }
            Err(err) => {
                last_error = format!("error sending request for url ({url}): {err}");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "Client under test {} never reached a non-genesis justified checkpoint",
        client.kind
    ) + &format!(
        " (last observed slot: {}, last error: {})",
        last_observed_slot
            .map(|slot| slot.to_string())
            .unwrap_or_else(|| "none".to_string()),
        if last_error.is_empty() {
            "none".to_string()
        } else {
            last_error
        }
    ))
}

fn http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("Unable to build HTTP client")
}

async fn load_finalized_genesis_validators(
    client: &Client,
) -> Result<Vec<HelperGenesisValidatorEntry>, String> {
    let http = http_client();
    let url = format!("http://{}:{}/hive/genesis", client.ip, HELPER_METADATA_PORT);
    let mut last_error = String::new();

    for _attempt in 0..30 {
        match http.get(&url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    match response.json::<HelperGenesisMetadata>().await {
                        Ok(metadata) => return Ok(metadata.genesis_validator_entries),
                        Err(err) => {
                            last_error = format!("Unable to decode helper genesis metadata: {err}")
                        }
                    }
                }
            }
            Err(err) => {
                last_error = format!("error sending request for url ({url}): {err}");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "Unable to load helper genesis metadata from {url}: {last_error}"
    ))
}
