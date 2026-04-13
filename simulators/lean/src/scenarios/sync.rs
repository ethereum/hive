use std::collections::HashMap;
use std::env;
use std::net::{IpAddr, UdpSocket};
use std::process::{Child, Command, Stdio};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::{lean_api_url, lean_environment, CheckpointResponse};
use hivesim::types::ClientDefinition;
use hivesim::{Client, Test};
use reqwest::{Client as HttpClient, Url};
use serde::Deserialize;
use tokio::time::sleep;

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
const LOCAL_HELPER_STARTUP_TIMEOUT_SECS: u64 = 120;
const LOCAL_HELPER_STARTUP_ATTEMPTS: u64 = 6;
const CLIENT_UNDER_TEST_STARTUP_ATTEMPTS: u64 = 3;
const HELPER_METADATA_PORT: u16 = 5053;
const LOCAL_HELPER_API_PORT: u16 = 5052;
const LOCAL_HELPER_ENTRYPOINT: &str = "/app/hive/leanspec-client.sh";
const LOCAL_HELPER_KIND: &str = "lean-spec-local-helper";
const SSZ_CONTENT_TYPE: &str = "application/octet-stream";

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
    _helper: RunningLocalLeanSpecHelper,
    pub client_under_test: Client,
    pub source_fork_choice: ForkChoiceSnapshot,
    pub client_checkpoint: Option<CheckpointResponse>,
}

struct RunningLocalLeanSpecHelper {
    child: Child,
    ip: IpAddr,
}

impl RunningLocalLeanSpecHelper {
    fn metadata_url(&self) -> String {
        format!("http://{}:{}/hive/genesis", self.ip, HELPER_METADATA_PORT)
    }

    fn fork_choice_url(&self) -> String {
        format!(
            "http://{}:{}/lean/v0/fork_choice",
            self.ip, LOCAL_HELPER_API_PORT
        )
    }

    fn checkpoint_sync_url(&self) -> String {
        format!("http://{}:{}", self.ip, LOCAL_HELPER_API_PORT)
    }

    fn bootnode_multiaddr(&self) -> String {
        format!(
            "/ip4/{}/udp/{}/quic-v1/p2p/{}",
            self.ip, LEAN_P2P_PORT, LEAN_SPEC_SOURCE_PEER_ID
        )
    }

    fn ensure_running(&mut self) -> Result<(), String> {
        match self.child.try_wait() {
            Ok(Some(status)) => Err(format!(
                "{LOCAL_HELPER_KIND} exited before the test completed with status {status}"
            )),
            Ok(None) => Ok(()),
            Err(err) => Err(format!(
                "Unable to inspect {LOCAL_HELPER_KIND} process status: {err}"
            )),
        }
    }
}

impl Drop for RunningLocalLeanSpecHelper {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
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
    test: &Test,
    test_data: &PostGenesisSyncTestData,
) -> PostGenesisSyncContext {
    let (mut helper, source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(test_data.genesis_time)
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to load finalized genesis validators from {LOCAL_HELPER_KIND}: {err}"
                )
            });
    let should_start_client_early =
        !test_data.use_checkpoint_sync && test_data.connect_client_to_lean_spec_mesh;
    let client_under_test_environment = client_under_test_environment(
        &helper,
        test_data.genesis_time,
        &source_genesis_validator_entries,
        test_data.use_checkpoint_sync,
        test_data.connect_client_to_lean_spec_mesh,
    );

    let client_under_test = if should_start_client_early {
        Some(
            start_client_under_test_with_retry(
                test,
                test_data.client_under_test.name.clone(),
                client_under_test_environment.clone(),
            )
            .await,
        )
    } else {
        None
    };

    let source_fork_choice = match wait_for_checkpoint_slot(
        &mut helper,
        test_data.source_checkpoint_kind,
        minimum_source_checkpoint_slot(test_data),
    )
    .await
    {
        Ok(source_fork_choice) => source_fork_choice,
        Err(err) => {
            if client_under_test.is_none() {
                let _ = test
                    .start_client(
                        test_data.client_under_test.name.clone(),
                        Some(client_under_test_environment.clone()),
                    )
                    .await;
            }

            panic!("{err}");
        }
    };

    if test_data.use_checkpoint_sync {
        wait_for_checkpoint_sync_state_ready(&mut helper)
            .await
            .unwrap_or_else(|err| panic!("{err}"));
    }

    let client_under_test = match client_under_test {
        Some(client_under_test) => client_under_test,
        None => {
            start_client_under_test_with_retry(
                test,
                test_data.client_under_test.name.clone(),
                client_under_test_environment,
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
        _helper: helper,
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
    checkpoint_source: &RunningLocalLeanSpecHelper,
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
            checkpoint_source.checkpoint_sync_url(),
        );
    }

    if connect_to_lean_spec_mesh {
        environment.insert(
            BOOTNODES_ENVIRONMENT_VARIABLE.to_string(),
            checkpoint_source.bootnode_multiaddr(),
        );
    }

    environment
}

fn minimum_source_checkpoint_slot(test_data: &PostGenesisSyncTestData) -> u64 {
    if test_data.use_checkpoint_sync {
        MIN_FINALIZED_SLOT_FOR_CHECKPOINT_SYNC
    } else {
        1
    }
}

fn panic_payload_to_string(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(error) = payload.downcast_ref::<&'static str>() {
        error.to_string()
    } else if let Some(error) = payload.downcast_ref::<String>() {
        error.clone()
    } else {
        "unknown panic payload".to_string()
    }
}

async fn start_client_under_test_with_retry(
    test: &Test,
    client_type: String,
    environment: HashMap<String, String>,
) -> Client {
    let mut last_error = None;

    for attempt in 1..=CLIENT_UNDER_TEST_STARTUP_ATTEMPTS {
        let test = test.clone();
        let client_type_for_attempt = client_type.clone();
        let environment_for_attempt = environment.clone();

        match tokio::spawn(async move {
            test.start_client(client_type_for_attempt, Some(environment_for_attempt))
                .await
        })
        .await
        {
            Ok(client) => return client,
            Err(error) if attempt < CLIENT_UNDER_TEST_STARTUP_ATTEMPTS => {
                let message = if error.is_panic() {
                    panic_payload_to_string(error.into_panic())
                } else {
                    error.to_string()
                };
                eprintln!(
                    "Retrying client-under-test startup for {} after attempt {} failed: {}",
                    client_type, attempt, message
                );
                last_error = Some(message);
                sleep(Duration::from_secs(1)).await;
            }
            Err(error) => {
                let message = if error.is_panic() {
                    panic_payload_to_string(error.into_panic())
                } else {
                    error.to_string()
                };
                panic!(
                    "Unable to start client under test {} after {} attempts: {}",
                    client_type, CLIENT_UNDER_TEST_STARTUP_ATTEMPTS, message
                );
            }
        }
    }

    panic!(
        "Unable to start client under test {} after {} attempts{}",
        client_type,
        CLIENT_UNDER_TEST_STARTUP_ATTEMPTS,
        last_error
            .map(|error| format!(": {error}"))
            .unwrap_or_default()
    );
}

async fn wait_for_checkpoint_slot(
    helper: &mut RunningLocalLeanSpecHelper,
    checkpoint_kind: SourceCheckpointKind,
    minimum_slot: u64,
) -> Result<ForkChoiceSnapshot, String> {
    let http = http_client();
    let url = helper.fork_choice_url();
    let checkpoint_name = match checkpoint_kind {
        SourceCheckpointKind::Justified => "justified",
        SourceCheckpointKind::Finalized => "finalized",
    };
    let mut last_error = String::new();
    let mut last_observed_slot = None;

    for _attempt in 0..POST_GENESIS_TIMEOUT_SECS {
        helper.ensure_running()?;
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
        "{LOCAL_HELPER_KIND} never reached {checkpoint_name} slot {} (last observed slot: {}, last error: {})",
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

async fn wait_for_checkpoint_sync_state_ready(
    helper: &mut RunningLocalLeanSpecHelper,
) -> Result<(), String> {
    let http = http_client();
    let url = format!(
        "http://{}:{}/lean/v0/states/finalized",
        helper.ip, LOCAL_HELPER_API_PORT
    );
    let mut last_error = String::new();
    let mut consecutive_successes = 0;

    for _attempt in 0..LOCAL_HELPER_STARTUP_TIMEOUT_SECS {
        helper.ensure_running()?;
        match http
            .get(&url)
            .header(reqwest::header::ACCEPT, SSZ_CONTENT_TYPE)
            .send()
            .await
        {
            Ok(response) => {
                let status = response.status();
                if status.is_success() {
                    consecutive_successes += 1;
                    if consecutive_successes >= 3 {
                        return Ok(());
                    }
                } else {
                    consecutive_successes = 0;
                    last_error = format!("received HTTP {status} from {url}");
                }
            }
            Err(err) => {
                consecutive_successes = 0;
                last_error = format!("error sending request for url ({url}): {err}");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "Checkpoint-sync source state endpoint never became ready at {url}: {last_error}"
    ))
}

fn http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("Unable to build HTTP client")
}

async fn load_finalized_genesis_validators(
    helper: &mut RunningLocalLeanSpecHelper,
) -> Result<Vec<HelperGenesisValidatorEntry>, String> {
    let http = http_client();
    let url = helper.metadata_url();
    let mut last_error = String::new();

    for _attempt in 0..LOCAL_HELPER_STARTUP_TIMEOUT_SECS {
        helper.ensure_running()?;
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

fn helper_startup_error_is_retryable(error: &str) -> bool {
    error.contains("exited before the test completed")
        || error.contains("SIGSEGV")
        || error.contains("signal: 11")
}

async fn start_local_lean_spec_helper_with_genesis_metadata(
    genesis_time: u64,
) -> Result<(RunningLocalLeanSpecHelper, Vec<HelperGenesisValidatorEntry>), String> {
    let mut last_error = None;

    for attempt in 1..=LOCAL_HELPER_STARTUP_ATTEMPTS {
        let mut helper = start_local_lean_spec_helper(genesis_time);
        match load_finalized_genesis_validators(&mut helper).await {
            Ok(source_genesis_validator_entries) => {
                return Ok((helper, source_genesis_validator_entries));
            }
            Err(error)
                if attempt < LOCAL_HELPER_STARTUP_ATTEMPTS
                    && helper_startup_error_is_retryable(&error) =>
            {
                eprintln!(
                    "Retrying {LOCAL_HELPER_KIND} startup after attempt {attempt} failed: {error}"
                );
                last_error = Some(error);
                drop(helper);
                sleep(Duration::from_secs(1)).await;
            }
            Err(error) => {
                return Err(error);
            }
        }
    }

    Err(last_error.unwrap_or_else(|| {
        format!(
            "Unable to start {LOCAL_HELPER_KIND} after {} attempts",
            LOCAL_HELPER_STARTUP_ATTEMPTS
        )
    }))
}

fn start_local_lean_spec_helper(genesis_time: u64) -> RunningLocalLeanSpecHelper {
    let mut command = Command::new(LOCAL_HELPER_ENTRYPOINT);
    command.stdout(Stdio::inherit());
    command.stderr(Stdio::inherit());

    for (key, value) in lean_spec_source_environment(genesis_time) {
        command.env(key, value);
    }

    let child = command.spawn().unwrap_or_else(|err| {
        panic!(
            "Unable to start local LeanSpec helper from {}: {err}",
            LOCAL_HELPER_ENTRYPOINT
        )
    });

    RunningLocalLeanSpecHelper {
        child,
        ip: simulator_container_ip(),
    }
}

fn simulator_container_ip() -> IpAddr {
    let simulator_url = env::var("HIVE_SIMULATOR")
        .expect("HIVE_SIMULATOR environment variable should be set inside the simulator");
    let url = Url::parse(&simulator_url).unwrap_or_else(|err| {
        panic!("Unable to parse HIVE_SIMULATOR URL `{simulator_url}`: {err}")
    });
    let host = url
        .host_str()
        .unwrap_or_else(|| panic!("HIVE_SIMULATOR URL `{simulator_url}` does not include a host"));
    let port = url.port_or_known_default().unwrap_or(80);
    let socket = UdpSocket::bind("0.0.0.0:0").unwrap_or_else(|err| {
        panic!("Unable to bind UDP socket to determine simulator container IP: {err}")
    });
    socket.connect((host, port)).unwrap_or_else(|err| {
        panic!(
            "Unable to connect UDP socket to HIVE_SIMULATOR host {}:{} to determine simulator container IP: {err}",
            host, port
        )
    });
    socket
        .local_addr()
        .unwrap_or_else(|err| {
            panic!("Unable to determine simulator container local socket address: {err}")
        })
        .ip()
}
