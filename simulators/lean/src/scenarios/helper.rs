//! Shared Lean scenario test helpers, including post-genesis sync setup.

use std::collections::HashMap;
use std::env;
use std::fs;
use std::future::Future;
use std::net::{IpAddr, UdpSocket};
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::{
    get_json_with_retry, lean_api_url, lean_environment, selected_lean_devnet, CheckpointResponse,
    LeanDevnet,
};
use alloy_primitives::B256;
use hivesim::types::ClientDefinition;
use hivesim::{types::TestResult, Client, Test};
use reqwest::{header::ACCEPT, Client as HttpClient, Url};
use serde::Deserialize;
use tokio::time::{sleep, timeout};

const CHECKPOINT_SYNC_URL_ENVIRONMENT_VARIABLE: &str = "HIVE_CHECKPOINT_SYNC_URL";
const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const LEAN_GENESIS_VALIDATORS_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_VALIDATORS";
const LEAN_GENESIS_VALIDATOR_ENTRIES_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const LEAN_HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_ADVERTISE_IP";
const LEAN_HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_GOSSIP_FORK_DIGEST";
const LEAN_HELPER_P2P_PORT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_P2P_PORT";
const LEAN_HELPER_API_PORT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_API_PORT";
const LEAN_HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_METADATA_PORT";
const LEAN_HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY";
const LEAN_RUNTIME_ASSET_ROOT_ENVIRONMENT_VARIABLE: &str = "LEAN_RUNTIME_ASSET_ROOT";
const LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_CLIENT_RUNTIME_ROLE";
const LEAN_CLIENT_RUNTIME_ROLE_OBSERVER: &str = "observer";
const LEAN_SPEC_SOURCE_NODE_ID: &str = "lean_spec_0";
const LEAN_SPEC_SOURCE_VALIDATORS: &str = "0,1,2";
const LEAN_SPEC_SOURCE_PEER_ID: &str = "16Uiu2HAmHzBkRq62mG95vsjKMuYQBezZCtjPXYWUoyVxMxi71aB3";
const DEFAULT_HELPER_GOSSIP_FORK_DIGEST: &str = "devnet0";
const DEVNET4_HELPER_GOSSIP_FORK_DIGEST: &str = "12345678";
const DEFAULT_HELPER_P2P_PORT: u16 = 9001;
const DEFAULT_HELPER_API_PORT: u16 = 5052;
const DEFAULT_HELPER_METADATA_PORT: u16 = 5053;
const LEAN_GENESIS_DELAY_SECS: u64 = 30;
const MESH_HELPER_GENESIS_BUFFER_PER_PEER_SECS: u64 = 10;
const POST_GENESIS_TIMEOUT_SECS: u64 = 600;
const MIN_FINALIZED_SLOT_FOR_CHECKPOINT_SYNC: u64 = 10;
const LOCAL_HELPER_STARTUP_TIMEOUT_SECS: u64 = 120;
const LOCAL_HELPER_METADATA_TIMEOUT_SECS: u64 = 60;
const LOCAL_HELPER_STARTUP_ATTEMPTS: u64 = 10;
const LOCAL_HELPER_RETRY_DELAY_SECS: u64 = 2;
const CLIENT_UNDER_TEST_STARTUP_ATTEMPTS: u64 = 3;
const CLIENT_UNDER_TEST_STARTUP_ATTEMPT_TIMEOUT_SECS: u64 = 120;
const CHECKPOINT_SYNC_CLIENT_READY_TIMEOUT_SECS: u64 = 30;
const MESH_HELPER_READY_TIMEOUT_SECS: u64 = 120;
const LIVE_HELPER_FORK_CHOICE_RETRY_ATTEMPTS: u64 = 10;
const LOCAL_HELPER_ENTRYPOINT: &str = "/app/hive/leanspec-client.sh";
const LOCAL_HELPER_KIND: &str = "lean-spec-local-helper";
const CLIENT_RUNTIME_ASSET_PREPARER: &str = "/app/hive/prepare_lean_client_assets.py";
const SSZ_CONTENT_TYPE: &str = "application/octet-stream";

#[derive(Debug, Deserialize)]
struct HelperGenesisValidatorEntry {
    attestation_public_key: String,
    proposal_public_key: Option<String>,
}

#[derive(Debug, Deserialize)]
struct HelperGenesisMetadata {
    genesis_time: u64,
    genesis_validator_entries: Vec<HelperGenesisValidatorEntry>,
    bootnode_enr: Option<String>,
    bootnode_multiaddr: Option<String>,
}

#[derive(Clone, Copy)]
pub(crate) enum ClientUnderTestRole {
    Validator,
    Observer,
}

#[derive(Clone, Copy)]
pub(crate) enum HelperGossipForkDigestProfile {
    LegacyDevnet0,
    SelectedDevnet,
}

#[derive(Clone)]
pub(crate) struct PostGenesisSyncTestData {
    pub client_under_test: ClientDefinition,
    pub genesis_time: u64,
    pub wait_for_client_justified_checkpoint: bool,
    pub use_checkpoint_sync: bool,
    pub connect_client_to_lean_spec_mesh: bool,
    pub client_role: ClientUnderTestRole,
    pub source_helper_validator_indices: Option<String>,
    pub helper_peer_count: usize,
    pub helper_fork_digest_profile: HelperGossipForkDigestProfile,
}

pub(crate) struct PostGenesisSyncContext {
    _helpers: RunningLocalLeanSpecHelperGroup,
    pub client_under_test: Client,
    pub source_fork_choice: ForkChoiceSnapshot,
    pub client_checkpoint: Option<CheckpointResponse>,
}

impl PostGenesisSyncContext {
    pub(crate) async fn load_live_helper_fork_choice(&mut self) -> ForkChoiceResponse {
        self._helpers
            .load_live_fork_choice()
            .await
            .unwrap_or_else(|err| panic!("{err}"))
    }
}

struct RunningLocalLeanSpecHelper {
    child: Child,
    ip: IpAddr,
    expected_genesis_time: u64,
    node_id: String,
    asset_root: PathBuf,
    p2p_port: u16,
    api_port: u16,
    metadata_port: u16,
    bootnode_enr: Option<String>,
    bootnode_multiaddr: Option<String>,
}

struct RunningLocalLeanSpecHelperGroup {
    source: RunningLocalLeanSpecHelper,
    mesh_peers: Vec<RunningLocalLeanSpecHelper>,
}

#[derive(Clone)]
struct LocalLeanSpecHelperConfig {
    node_id: String,
    validator_indices: String,
    genesis_time: u64,
    bootnodes: Option<String>,
    is_aggregator: bool,
    gossip_fork_digest: String,
    p2p_port: u16,
    api_port: u16,
    metadata_port: u16,
    identity_private_key_hex: Option<String>,
}

struct CheckpointSyncClientStart<'a> {
    test: &'a Test,
    helper: &'a mut RunningLocalLeanSpecHelper,
    source_fork_choice: &'a mut ForkChoiceSnapshot,
    helper_config: LocalLeanSpecHelperConfig,
    client_type: String,
    environment: HashMap<String, String>,
    minimum_slot: u64,
}

impl RunningLocalLeanSpecHelperGroup {
    fn bootnodes_for_client(&self, client_type: &str) -> String {
        std::iter::once(&self.source)
            .chain(self.mesh_peers.iter())
            .map(|helper| helper.bootnode_for_client(client_type))
            .collect::<Vec<_>>()
            .join(",")
    }

    async fn load_live_fork_choice(&mut self) -> Result<ForkChoiceResponse, String> {
        let mut last_errors = Vec::new();

        for attempt in 1..=LIVE_HELPER_FORK_CHOICE_RETRY_ATTEMPTS {
            let mut best_fork_choice = None;
            let mut attempt_errors = Vec::new();

            match self.source.try_load_fork_choice().await {
                Ok(fork_choice) => best_fork_choice = Some(fork_choice),
                Err(err) => attempt_errors.push(err),
            }

            for helper in &mut self.mesh_peers {
                match helper.try_load_fork_choice().await {
                    Ok(fork_choice) => {
                        let should_replace = best_fork_choice
                            .as_ref()
                            .map(|current| is_better_fork_choice(&fork_choice, current))
                            .unwrap_or(true);
                        if should_replace {
                            best_fork_choice = Some(fork_choice);
                        }
                    }
                    Err(err) => attempt_errors.push(err),
                }
            }

            if let Some(fork_choice) = best_fork_choice {
                return Ok(fork_choice);
            }

            last_errors = attempt_errors;
            if attempt < LIVE_HELPER_FORK_CHOICE_RETRY_ATTEMPTS {
                sleep(Duration::from_secs(1)).await;
            }
        }

        Err(format!(
            "Unable to load a live forkchoice response from any local LeanSpec helper after {} attempts: {}",
            LIVE_HELPER_FORK_CHOICE_RETRY_ATTEMPTS,
            if last_errors.is_empty() {
                "no helper responses collected".to_string()
            } else {
                last_errors.join(" | ")
            }
        ))
    }
}

impl LocalLeanSpecHelperConfig {
    fn source(
        genesis_time: u64,
        gossip_fork_digest: String,
        validator_indices: Option<String>,
    ) -> Self {
        Self {
            node_id: LEAN_SPEC_SOURCE_NODE_ID.to_string(),
            validator_indices: validator_indices
                .unwrap_or_else(|| LEAN_SPEC_SOURCE_VALIDATORS.to_string()),
            genesis_time,
            bootnodes: None,
            is_aggregator: true,
            gossip_fork_digest,
            p2p_port: DEFAULT_HELPER_P2P_PORT,
            api_port: DEFAULT_HELPER_API_PORT,
            metadata_port: DEFAULT_HELPER_METADATA_PORT,
            identity_private_key_hex: None,
        }
    }

    fn mesh_peer(
        mesh_index: usize,
        genesis_time: u64,
        bootnode: String,
        gossip_fork_digest: String,
    ) -> Self {
        let mesh_index = mesh_index as u16;
        Self {
            node_id: format!("lean_spec_mesh_{mesh_index}"),
            validator_indices: String::new(),
            genesis_time,
            bootnodes: Some(bootnode),
            is_aggregator: false,
            gossip_fork_digest,
            p2p_port: DEFAULT_HELPER_P2P_PORT + mesh_index,
            api_port: DEFAULT_HELPER_API_PORT + (mesh_index * 2),
            metadata_port: DEFAULT_HELPER_METADATA_PORT + (mesh_index * 2),
            identity_private_key_hex: Some(format!("{:064x}", mesh_index as u64 + 1)),
        }
    }
}

impl RunningLocalLeanSpecHelper {
    fn metadata_url(&self) -> String {
        format!("http://{}:{}/hive/genesis", self.ip, self.metadata_port)
    }

    fn fork_choice_url(&self) -> String {
        format!("http://{}:{}/lean/v0/fork_choice", self.ip, self.api_port)
    }

    fn checkpoint_sync_url(&self) -> String {
        format!(
            "http://{}:{}/lean/v0/states/finalized",
            self.ip, self.api_port
        )
    }

    fn fallback_bootnode_multiaddr(&self) -> String {
        format!(
            "/ip4/{}/udp/{}/quic-v1/p2p/{}",
            self.ip, self.p2p_port, LEAN_SPEC_SOURCE_PEER_ID
        )
    }

    fn bootnode_multiaddr(&self) -> String {
        self.bootnode_multiaddr
            .clone()
            .unwrap_or_else(|| self.fallback_bootnode_multiaddr())
    }

    fn bootnode_for_client(&self, client_type: &str) -> String {
        if client_type.starts_with("ethlambda") || client_type.starts_with("zeam") {
            return self
                .bootnode_enr
                .clone()
                .unwrap_or_else(|| self.bootnode_multiaddr());
        }

        self.bootnode_multiaddr()
    }

    async fn try_load_fork_choice(&mut self) -> Result<ForkChoiceResponse, String> {
        self.ensure_running()?;

        let url = self.fork_choice_url();
        let response = http_client()
            .get(&url)
            .send()
            .await
            .map_err(|err| format!("{} {} request failed: {err}", self.node_id, url))?;
        let status = response.status();
        if !status.is_success() {
            return Err(format!("{} {} returned HTTP {}", self.node_id, url, status));
        }

        response.json::<ForkChoiceResponse>().await.map_err(|err| {
            format!(
                "{} {} returned an invalid forkchoice response: {err}",
                self.node_id, url
            )
        })
    }

    fn ensure_running(&mut self) -> Result<(), String> {
        match self.child.try_wait() {
            Ok(Some(status)) => Err(format!(
                "{LOCAL_HELPER_KIND} `{}` exited before the test completed with status {status}",
                self.node_id
            )),
            Ok(None) => Ok(()),
            Err(err) => Err(format!(
                "Unable to inspect {LOCAL_HELPER_KIND} `{}` process status: {err}",
                self.node_id
            )),
        }
    }
}

impl Drop for RunningLocalLeanSpecHelper {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_dir_all(&self.asset_root);
    }
}

#[derive(Debug, Deserialize)]
pub(crate) struct ForkChoiceSnapshot {
    pub justified: CheckpointResponse,
    pub finalized: CheckpointResponse,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ForkChoiceNodeResponse {
    pub root: B256,
    pub slot: u64,
    pub parent_root: B256,
    pub proposer_index: u64,
    pub weight: u64,
}

#[derive(Debug, Deserialize)]
pub(crate) struct ForkChoiceResponse {
    #[serde(default)]
    pub nodes: Vec<ForkChoiceNodeResponse>,
    pub head: B256,
    pub justified: CheckpointResponse,
    pub finalized: CheckpointResponse,
    #[serde(default)]
    pub safe_target: B256,
    #[serde(default)]
    pub validator_count: u64,
}

pub(crate) fn fork_choice_head_slot(fork_choice: &ForkChoiceResponse) -> u64 {
    fork_choice
        .nodes
        .iter()
        .find(|node| node.root == fork_choice.head)
        .map(|node| node.slot)
        .unwrap_or_else(|| {
            let max_node_slot = fork_choice
                .nodes
                .iter()
                .map(|node| node.slot)
                .max()
                .unwrap_or(0);
            if max_node_slot > 0 {
                return max_node_slot;
            }

            if fork_choice.head == fork_choice.justified.root {
                return fork_choice.justified.slot;
            }
            if fork_choice.head == fork_choice.finalized.root {
                return fork_choice.finalized.slot;
            }

            fork_choice.justified.slot.max(fork_choice.finalized.slot)
        })
}

fn is_better_fork_choice(candidate: &ForkChoiceResponse, current: &ForkChoiceResponse) -> bool {
    let candidate_key = (
        fork_choice_head_slot(candidate),
        candidate.finalized.slot,
        candidate.justified.slot,
    );
    let current_key = (
        fork_choice_head_slot(current),
        current.finalized.slot,
        current.justified.slot,
    );

    candidate_key > current_key
}

#[derive(Debug, Deserialize)]
struct ForkChoiceNodeSlot {
    slot: u64,
}

#[derive(Debug, Deserialize)]
struct CheckpointSyncForkChoiceSnapshot {
    justified: CheckpointResponse,
    finalized: CheckpointResponse,
    #[serde(default)]
    nodes: Vec<ForkChoiceNodeSlot>,
}

pub(crate) type AsyncLeanDataTestFunc<T> =
    fn(&mut Test, T) -> Pin<Box<dyn Future<Output = ()> + Send + '_>>;
pub(crate) type ClientEnvironments = Option<Vec<Option<HashMap<String, String>>>>;
pub(crate) type ClientFiles = Option<Vec<Option<HashMap<String, Vec<u8>>>>>;
pub(crate) type ClientRuntimeSetup = (ClientEnvironments, ClientFiles);

pub(crate) struct TimedDataTestSpec<T> {
    pub name: String,
    pub description: String,
    pub always_run: bool,
    pub client_name: String,
    pub timeout_duration: Duration,
    pub test_data: T,
}

pub(crate) async fn load_fork_choice_response(client: &Client) -> ForkChoiceResponse {
    let http = http_client();
    get_json_with_retry(&http, &lean_api_url(client, "/lean/v0/fork_choice")).await
}

pub(crate) fn expect_single_client(clients: Vec<Client>) -> Client {
    clients
        .into_iter()
        .next()
        .expect("NClientTestSpec should start exactly one client")
}

pub(crate) fn lean_single_client_runtime_setup(client_type: &str) -> ClientRuntimeSetup {
    let environment = lean_environment();
    let files = prepare_client_runtime_files(client_type, &environment)
        .unwrap_or_else(|err| panic!("Unable to prepare runtime assets for {client_type}: {err}"));

    (Some(vec![Some(environment)]), Some(vec![Some(files)]))
}

pub(crate) struct LiveHelperSingleClientRuntimeSetup {
    pub environment: HashMap<String, String>,
    pub files: HashMap<String, Vec<u8>>,
    _helpers: RunningLocalLeanSpecHelperGroup,
}

pub(crate) async fn lean_single_client_runtime_setup_with_live_helper(
    client_type: &str,
    genesis_time: u64,
    minimum_source_slot: u64,
    helper_fork_digest_profile: HelperGossipForkDigestProfile,
    client_role: ClientUnderTestRole,
    use_checkpoint_sync: bool,
    connect_to_lean_spec_mesh: bool,
) -> LiveHelperSingleClientRuntimeSetup {
    let helper_fork_digest = helper_gossip_fork_digest(helper_fork_digest_profile);
    let source_helper_config =
        LocalLeanSpecHelperConfig::source(genesis_time, helper_fork_digest, None);
    let (source_helper, source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(&source_helper_config)
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to load finalized genesis validators from {LOCAL_HELPER_KIND}: {err}"
                )
            });
    let mut helpers = RunningLocalLeanSpecHelperGroup {
        source: source_helper,
        mesh_peers: Vec::new(),
    };

    wait_for_checkpoint_slot_with_retry(
        &mut helpers.source,
        minimum_source_slot,
        &source_helper_config,
    )
    .await
    .unwrap_or_else(|err| panic!("{err}"));

    let environment = client_under_test_environment(
        &helpers,
        client_type,
        genesis_time,
        &source_genesis_validator_entries,
        use_checkpoint_sync,
        connect_to_lean_spec_mesh,
        client_role,
    );
    let files = prepare_client_runtime_files(client_type, &environment)
        .unwrap_or_else(|err| panic!("Unable to prepare runtime assets for {client_type}: {err}"));

    LiveHelperSingleClientRuntimeSetup {
        environment,
        files,
        _helpers: helpers,
    }
}

fn extract_data_test_result(join_handle: Result<(), tokio::task::JoinError>) -> TestResult {
    match join_handle {
        Ok(()) => TestResult {
            pass: true,
            details: String::new(),
        },
        Err(err) => TestResult {
            pass: false,
            details: panic_payload_to_string(err.into_panic()),
        },
    }
}

fn annotate_failed_client(mut test_result: TestResult, client_name: &str) -> TestResult {
    if test_result.pass {
        return test_result;
    }

    if test_result.details.is_empty() {
        test_result.details = format!("client {client_name} failed without an error message");
    } else if !test_result.details.contains(client_name) {
        test_result.details = format!("client {client_name}: {}", test_result.details);
    }

    test_result
}

pub(crate) async fn run_data_test<T: Send + 'static>(
    host_test: &Test,
    name: String,
    description: String,
    always_run: bool,
    test_data: T,
    func: AsyncLeanDataTestFunc<T>,
) {
    if let Some(test_match) = host_test.sim.test_matcher.clone() {
        if !always_run && !test_match.match_test(&host_test.suite.name, &name) {
            return;
        }
    }

    let test_id = host_test
        .sim
        .start_test(host_test.suite_id, name, description)
        .await;
    let suite_id = host_test.suite_id;
    let suite = host_test.suite.clone();
    let simulation = host_test.sim.clone();

    let test_result = extract_data_test_result(
        tokio::spawn(async move {
            let test = &mut Test {
                sim: simulation,
                test_id,
                suite,
                suite_id,
                result: Default::default(),
            };

            test.result.pass = true;
            (func)(test, test_data).await;
        })
        .await,
    );

    host_test.sim.end_test(suite_id, test_id, test_result).await;
}

pub(crate) async fn run_data_test_with_timeout<T: Send + 'static>(
    host_test: &Test,
    spec: TimedDataTestSpec<T>,
    func: AsyncLeanDataTestFunc<T>,
) {
    let TimedDataTestSpec {
        name,
        description,
        always_run,
        client_name,
        timeout_duration,
        test_data,
    } = spec;

    if let Some(test_match) = host_test.sim.test_matcher.clone() {
        if !always_run && !test_match.match_test(&host_test.suite.name, &name) {
            return;
        }
    }

    let test_id = host_test
        .sim
        .start_test(host_test.suite_id, name, description)
        .await;
    let suite_id = host_test.suite_id;
    let suite = host_test.suite.clone();
    let simulation = host_test.sim.clone();

    let mut join_handle = tokio::spawn(async move {
        let test = &mut Test {
            sim: simulation,
            test_id,
            suite,
            suite_id,
            result: Default::default(),
        };

        test.result.pass = true;
        (func)(test, test_data).await;
    });

    let test_result = match timeout(timeout_duration, &mut join_handle).await {
        Ok(join_result) => {
            annotate_failed_client(extract_data_test_result(join_result), &client_name)
        }
        Err(_) => {
            join_handle.abort();
            let _ = join_handle.await;
            TestResult {
                pass: false,
                details: format!(
                    "client {}: test exceeded timeout of {} seconds",
                    client_name,
                    timeout_duration.as_secs()
                ),
            }
        }
    };

    host_test.sim.end_test(suite_id, test_id, test_result).await;
}

pub(crate) fn default_genesis_time() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time is before UNIX_EPOCH")
        .as_secs()
        + LEAN_GENESIS_DELAY_SECS
}

fn adjusted_genesis_time_for_helper_mesh(
    requested_genesis_time: u64,
    helper_peer_count: usize,
    connect_client_to_lean_spec_mesh: bool,
) -> u64 {
    if !connect_client_to_lean_spec_mesh || helper_peer_count <= 1 {
        return requested_genesis_time;
    }

    // Mesh helpers do not backfill the earliest chain history on startup, so they
    // need a fresh pre-genesis launch window to observe the canonical chain from
    // the beginning instead of joining after the source helper is already live.
    let helper_startup_buffer =
        ((helper_peer_count - 1) as u64) * MESH_HELPER_GENESIS_BUFFER_PER_PEER_SECS;
    requested_genesis_time.max(default_genesis_time() + helper_startup_buffer)
}

fn helper_gossip_fork_digest(profile: HelperGossipForkDigestProfile) -> String {
    match profile {
        HelperGossipForkDigestProfile::LegacyDevnet0 => {
            DEFAULT_HELPER_GOSSIP_FORK_DIGEST.to_string()
        }
        HelperGossipForkDigestProfile::SelectedDevnet => match selected_lean_devnet() {
            LeanDevnet::Devnet3 => LeanDevnet::Devnet3.to_string(),
            LeanDevnet::Devnet4 => DEVNET4_HELPER_GOSSIP_FORK_DIGEST.to_string(),
        },
    }
}

pub(crate) async fn start_post_genesis_sync_context(
    test: &Test,
    test_data: &PostGenesisSyncTestData,
) -> PostGenesisSyncContext {
    let helper_peer_count = test_data.helper_peer_count.max(1);
    let genesis_time = adjusted_genesis_time_for_helper_mesh(
        test_data.genesis_time,
        helper_peer_count,
        test_data.connect_client_to_lean_spec_mesh,
    );
    let helper_fork_digest = helper_gossip_fork_digest(test_data.helper_fork_digest_profile);
    let source_helper_config = LocalLeanSpecHelperConfig::source(
        genesis_time,
        helper_fork_digest.clone(),
        test_data.source_helper_validator_indices.clone(),
    );
    let (source_helper, source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(&source_helper_config)
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to load finalized genesis validators from {LOCAL_HELPER_KIND}: {err}"
                )
            });
    let mesh_peers = if test_data.connect_client_to_lean_spec_mesh && helper_peer_count > 1 {
        start_mesh_helpers(
            &source_helper,
            genesis_time,
            helper_peer_count - 1,
            helper_fork_digest.clone(),
        )
        .await
        .unwrap_or_else(|err| panic!("{err}"))
    } else {
        Vec::new()
    };
    let mut helpers = RunningLocalLeanSpecHelperGroup {
        source: source_helper,
        mesh_peers,
    };
    let should_start_client_early =
        !test_data.use_checkpoint_sync && test_data.connect_client_to_lean_spec_mesh;
    let initial_client_under_test_environment = client_under_test_environment(
        &helpers,
        &test_data.client_under_test.name,
        genesis_time,
        &source_genesis_validator_entries,
        test_data.use_checkpoint_sync,
        test_data.connect_client_to_lean_spec_mesh,
        test_data.client_role,
    );

    let client_under_test = if should_start_client_early {
        Some(
            start_client_under_test_with_retry(
                test,
                test_data.client_under_test.name.clone(),
                initial_client_under_test_environment.clone(),
            )
            .await,
        )
    } else {
        None
    };

    let (mut source_fork_choice, mut source_helper_restarted) =
        match wait_for_checkpoint_slot_with_retry(
            &mut helpers.source,
            minimum_source_checkpoint_slot(test_data),
            &source_helper_config,
        )
        .await
        {
            Ok(source_fork_choice) => source_fork_choice,
            Err(err) => {
                if client_under_test.is_none() && !helper_startup_error_is_retryable(&err) {
                    let files = prepare_client_runtime_files(
                    &test_data.client_under_test.name,
                    &initial_client_under_test_environment,
                )
                .unwrap_or_else(|prep_err| {
                    panic!(
                        "Unable to prepare runtime assets for {} after checkpoint wait failure: {prep_err}",
                        test_data.client_under_test.name
                    )
                });
                    let _ = test
                        .start_client_with_files(
                            test_data.client_under_test.name.clone(),
                            Some(initial_client_under_test_environment.clone()),
                            Some(files),
                        )
                        .await;
                }

                panic!("{err}");
            }
        };

    if test_data.use_checkpoint_sync {
        source_helper_restarted |= ensure_checkpoint_sync_source_ready(
            &mut helpers.source,
            &mut source_fork_choice,
            &source_helper_config,
            minimum_source_checkpoint_slot(test_data),
        )
        .await
        .unwrap_or_else(|err| panic!("{err}"));
    }

    if !test_data.use_checkpoint_sync && !helpers.mesh_peers.is_empty() {
        wait_for_mesh_helpers_to_reach_post_genesis(&mut helpers.mesh_peers)
            .await
            .unwrap_or_else(|err| panic!("{err}"));
    }

    if test_data.use_checkpoint_sync
        && test_data.connect_client_to_lean_spec_mesh
        && helper_peer_count > 1
    {
        if source_helper_restarted {
            helpers.mesh_peers = start_mesh_helpers(
                &helpers.source,
                genesis_time,
                helper_peer_count - 1,
                helper_fork_digest.clone(),
            )
            .await
            .unwrap_or_else(|err| panic!("{err}"));
        }

        if let Err(err) =
            wait_for_any_mesh_helper_to_reach_post_genesis(&mut helpers.mesh_peers).await
        {
            eprintln!("Continuing checkpoint-sync test despite auxiliary helper lag: {err}");
        }
    }

    let client_under_test = match client_under_test {
        Some(client_under_test) => client_under_test,
        None if test_data.use_checkpoint_sync => {
            let checkpoint_sync_client_environment = client_under_test_environment(
                &helpers,
                &test_data.client_under_test.name,
                genesis_time,
                &source_genesis_validator_entries,
                test_data.use_checkpoint_sync,
                test_data.connect_client_to_lean_spec_mesh,
                test_data.client_role,
            );
            start_checkpoint_sync_client_under_test_with_retry(CheckpointSyncClientStart {
                test,
                helper: &mut helpers.source,
                source_fork_choice: &mut source_fork_choice,
                helper_config: source_helper_config.clone(),
                client_type: test_data.client_under_test.name.clone(),
                environment: checkpoint_sync_client_environment,
                minimum_slot: minimum_source_checkpoint_slot(test_data),
            })
            .await
        }
        None => {
            start_client_under_test_with_retry(
                test,
                test_data.client_under_test.name.clone(),
                initial_client_under_test_environment,
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
        _helpers: helpers,
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

fn local_lean_spec_helper_environment(
    helper_config: &LocalLeanSpecHelperConfig,
) -> HashMap<String, String> {
    let mut environment = lean_spec_environment(
        &helper_config.node_id,
        &helper_config.validator_indices,
        helper_config.genesis_time,
        helper_config.bootnodes.clone(),
        helper_config.is_aggregator,
    );
    environment.insert(
        LEAN_HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE.to_string(),
        helper_config.gossip_fork_digest.clone(),
    );
    environment.insert(
        LEAN_HELPER_P2P_PORT_ENVIRONMENT_VARIABLE.to_string(),
        helper_config.p2p_port.to_string(),
    );
    environment.insert(
        LEAN_HELPER_API_PORT_ENVIRONMENT_VARIABLE.to_string(),
        helper_config.api_port.to_string(),
    );
    environment.insert(
        LEAN_HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE.to_string(),
        helper_config.metadata_port.to_string(),
    );
    if let Some(identity_private_key_hex) = &helper_config.identity_private_key_hex {
        environment.insert(
            LEAN_HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(),
            identity_private_key_hex.clone(),
        );
    }

    environment
}

fn client_under_test_environment(
    helpers: &RunningLocalLeanSpecHelperGroup,
    client_type: &str,
    genesis_time: u64,
    source_genesis_validator_entries: &[HelperGenesisValidatorEntry],
    use_checkpoint_sync: bool,
    connect_to_lean_spec_mesh: bool,
    client_role: ClientUnderTestRole,
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
            helpers.source.checkpoint_sync_url(),
        );
    }

    if connect_to_lean_spec_mesh
        || (use_checkpoint_sync && client_type.starts_with("grandine_lean"))
    {
        environment.insert(
            BOOTNODES_ENVIRONMENT_VARIABLE.to_string(),
            helpers.bootnodes_for_client(client_type),
        );
    }

    if matches!(client_role, ClientUnderTestRole::Observer) {
        environment.insert(
            LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE.to_string(),
            LEAN_CLIENT_RUNTIME_ROLE_OBSERVER.to_string(),
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

fn lean_client_kind(client_type: &str) -> Result<&'static str, String> {
    for candidate in [
        "ethlambda",
        "grandine_lean",
        "zeam",
        "lantern",
        "qlean",
        "ream",
        "gean",
        "nlean",
    ] {
        if client_type.starts_with(candidate) {
            return Ok(candidate);
        }
    }
    Err(format!(
        "unsupported lean client type for runtime asset preparation: {client_type}"
    ))
}

fn client_runtime_asset_root(client_kind: &str) -> String {
    format!("/tmp/{client_kind}-runtime")
}

fn local_client_runtime_asset_root(client_kind: &str) -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    env::temp_dir().join(format!("lean-client-assets-{client_kind}-{timestamp}"))
}

fn local_helper_runtime_asset_root(node_id: &str) -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    env::temp_dir().join(format!("lean-spec-helper-{node_id}-{timestamp}"))
}

fn collect_client_runtime_files(
    source_root: &Path,
    current_dir: &Path,
    container_root: &str,
    files: &mut HashMap<String, Vec<u8>>,
) -> Result<(), String> {
    for entry in fs::read_dir(current_dir)
        .map_err(|err| format!("Unable to read prepared asset dir {current_dir:?}: {err}"))?
    {
        let entry = entry.map_err(|err| {
            format!("Unable to inspect entry inside prepared asset dir {current_dir:?}: {err}")
        })?;
        let entry_path = entry.path();
        if entry_path.is_dir() {
            collect_client_runtime_files(source_root, &entry_path, container_root, files)?;
            continue;
        }

        let relative_path = entry_path.strip_prefix(source_root).map_err(|err| {
            format!("Unable to derive relative asset path for {entry_path:?}: {err}")
        })?;
        let file_contents = fs::read(&entry_path)
            .map_err(|err| format!("Unable to read prepared asset file {entry_path:?}: {err}"))?;
        let relative_path = relative_path.to_string_lossy().replace('\\', "/");
        let container_path = format!("{container_root}/{relative_path}");

        files.insert(container_path, file_contents);
    }
    Ok(())
}

pub(crate) fn prepare_client_runtime_files(
    client_type: &str,
    environment: &HashMap<String, String>,
) -> Result<HashMap<String, Vec<u8>>, String> {
    let client_kind = lean_client_kind(client_type)?;
    let local_root = local_client_runtime_asset_root(client_kind);
    let container_root = client_runtime_asset_root(client_kind);

    let mut command = Command::new("python3");
    command.arg(CLIENT_RUNTIME_ASSET_PREPARER);
    command.env("LEAN_CLIENT_KIND", client_kind);
    command.env("LEAN_RUNTIME_ASSET_ROOT", &local_root);
    for (key, value) in environment {
        command.env(key, value);
    }

    let output = command.output().map_err(|err| {
        format!("Unable to execute {CLIENT_RUNTIME_ASSET_PREPARER} for {client_type}: {err}")
    })?;
    if !output.status.success() {
        let stdout = String::from_utf8_lossy(&output.stdout);
        let stderr = String::from_utf8_lossy(&output.stderr);
        let _ = fs::remove_dir_all(&local_root);
        return Err(format!(
            "{CLIENT_RUNTIME_ASSET_PREPARER} failed for {client_type} with status {}.\nstdout:\n{}\nstderr:\n{}",
            output.status, stdout, stderr
        ));
    }

    let mut files = HashMap::new();
    collect_client_runtime_files(&local_root, &local_root, &container_root, &mut files)?;
    let _ = fs::remove_dir_all(&local_root);
    Ok(files)
}

fn panic_payload_to_string(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(error) = payload.downcast_ref::<&'static str>() {
        error.to_string()
    } else if let Some(error) = payload.downcast_ref::<String>() {
        error.clone()
    } else {
        format!("unknown panic payload: {payload:?}")
    }
}

async fn start_client_under_test_attempt(
    test: Test,
    client_type: String,
    environment: HashMap<String, String>,
    files: HashMap<String, Vec<u8>>,
) -> Result<Client, String> {
    let mut handle = tokio::spawn(async move {
        test.start_client_with_files(client_type, Some(environment), Some(files))
            .await
    });

    match timeout(
        Duration::from_secs(CLIENT_UNDER_TEST_STARTUP_ATTEMPT_TIMEOUT_SECS),
        &mut handle,
    )
    .await
    {
        Ok(Ok(client)) => Ok(client),
        Ok(Err(error)) => {
            if error.is_panic() {
                Err(panic_payload_to_string(error.into_panic()))
            } else {
                Err(error.to_string())
            }
        }
        Err(_) => {
            handle.abort();
            let _ = handle.await;
            Err(format!(
                "startup attempt exceeded {} seconds",
                CLIENT_UNDER_TEST_STARTUP_ATTEMPT_TIMEOUT_SECS
            ))
        }
    }
}

async fn start_client_under_test_with_retry(
    test: &Test,
    client_type: String,
    environment: HashMap<String, String>,
) -> Client {
    let files = prepare_client_runtime_files(&client_type, &environment)
        .unwrap_or_else(|err| panic!("Unable to prepare runtime assets for {client_type}: {err}"));
    let mut last_error = None;

    for attempt in 1..=CLIENT_UNDER_TEST_STARTUP_ATTEMPTS {
        let test = test.clone();
        let client_type_for_attempt = client_type.clone();
        let environment_for_attempt = environment.clone();
        let files_for_attempt = files.clone();

        match start_client_under_test_attempt(
            test,
            client_type_for_attempt,
            environment_for_attempt,
            files_for_attempt,
        )
        .await
        {
            Ok(client) => return client,
            Err(message) if attempt < CLIENT_UNDER_TEST_STARTUP_ATTEMPTS => {
                eprintln!(
                    "Retrying client-under-test startup for {} after attempt {} failed: {}",
                    client_type, attempt, message
                );
                last_error = Some(message);
                sleep(Duration::from_secs(1)).await;
            }
            Err(message) => {
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

async fn start_checkpoint_sync_client_under_test_with_retry(
    params: CheckpointSyncClientStart<'_>,
) -> Client {
    let files = prepare_client_runtime_files(&params.client_type, &params.environment)
        .unwrap_or_else(|err| {
            panic!(
                "Unable to prepare runtime assets for {}: {err}",
                params.client_type
            )
        });
    let mut last_error = None;

    for attempt in 1..=CLIENT_UNDER_TEST_STARTUP_ATTEMPTS {
        let _ = ensure_checkpoint_sync_source_ready(
            params.helper,
            params.source_fork_choice,
            &params.helper_config,
            params.minimum_slot,
        )
        .await
        .unwrap_or_else(|err| {
            panic!(
                "Checkpoint-sync source state endpoint was not ready for {} before startup attempt {}: {}",
                params.client_type, attempt, err
            )
        });
        sleep(Duration::from_secs(1)).await;

        let test = params.test.clone();
        let client_type_for_attempt = params.client_type.clone();
        let environment_for_attempt = params.environment.clone();
        let files_for_attempt = files.clone();

        match start_client_under_test_attempt(
            test,
            client_type_for_attempt,
            environment_for_attempt,
            files_for_attempt,
        )
        .await
        {
            Ok(client) => match wait_for_checkpoint_sync_client_post_genesis(&client).await {
                Ok(()) => return client,
                Err(error) if attempt < CLIENT_UNDER_TEST_STARTUP_ATTEMPTS => {
                    eprintln!(
                        "Retrying checkpoint-sync client-under-test startup for {} after attempt {} never reached a post-genesis forkchoice state: {}",
                        params.client_type, attempt, error
                    );
                    last_error = Some(error);
                    drop(client);
                    sleep(Duration::from_secs(1)).await;
                }
                Err(error) => {
                    panic!(
                        "Unable to start checkpoint-sync client under test {} after {} attempts: {}",
                        params.client_type, CLIENT_UNDER_TEST_STARTUP_ATTEMPTS, error
                    );
                }
            },
            Err(message) if attempt < CLIENT_UNDER_TEST_STARTUP_ATTEMPTS => {
                eprintln!(
                    "Retrying checkpoint-sync client-under-test startup for {} after attempt {} failed: {}",
                    params.client_type, attempt, message
                );
                last_error = Some(message);
                sleep(Duration::from_secs(1)).await;
            }
            Err(message) => {
                panic!(
                    "Unable to start checkpoint-sync client under test {} after {} attempts: {}",
                    params.client_type, CLIENT_UNDER_TEST_STARTUP_ATTEMPTS, message
                );
            }
        }
    }

    panic!(
        "Unable to start checkpoint-sync client under test {} after {} attempts{}",
        params.client_type,
        CLIENT_UNDER_TEST_STARTUP_ATTEMPTS,
        last_error
            .map(|error| format!(": {error}"))
            .unwrap_or_default()
    );
}

async fn ensure_checkpoint_sync_source_ready(
    helper: &mut RunningLocalLeanSpecHelper,
    source_fork_choice: &mut ForkChoiceSnapshot,
    helper_config: &LocalLeanSpecHelperConfig,
    minimum_slot: u64,
) -> Result<bool, String> {
    match wait_for_checkpoint_sync_state_ready(helper).await {
        Ok(()) => Ok(false),
        Err(error) if helper_startup_error_is_retryable(&error) => {
            eprintln!(
                "Restarting {LOCAL_HELPER_KIND} after checkpoint-sync readiness failure: {error}"
            );
            refresh_checkpoint_sync_source(helper, source_fork_choice, helper_config, minimum_slot)
                .await
        }
        Err(error) => Err(error),
    }
}

async fn refresh_checkpoint_sync_source(
    helper: &mut RunningLocalLeanSpecHelper,
    source_fork_choice: &mut ForkChoiceSnapshot,
    helper_config: &LocalLeanSpecHelperConfig,
    minimum_slot: u64,
) -> Result<bool, String> {
    let (mut restarted_helper, _source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(helper_config).await?;
    let refreshed_source_fork_choice =
        wait_for_checkpoint_slot_with_retry(&mut restarted_helper, minimum_slot, helper_config)
            .await?
            .0;
    wait_for_checkpoint_sync_state_ready(&mut restarted_helper).await?;

    *helper = restarted_helper;
    *source_fork_choice = refreshed_source_fork_choice;
    Ok(true)
}

async fn wait_for_checkpoint_slot(
    helper: &mut RunningLocalLeanSpecHelper,
    minimum_slot: u64,
) -> Result<ForkChoiceSnapshot, String> {
    let http = http_client();
    let url = helper.fork_choice_url();
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
                            let checkpoint = &fork_choice.finalized;
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
        "{LOCAL_HELPER_KIND} never reached finalized slot {} (last observed slot: {}, last error: {})",
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

async fn wait_for_checkpoint_slot_with_retry(
    helper: &mut RunningLocalLeanSpecHelper,
    minimum_slot: u64,
    helper_config: &LocalLeanSpecHelperConfig,
) -> Result<(ForkChoiceSnapshot, bool), String> {
    let mut last_error = None;
    let mut helper_restarted = false;

    for attempt in 1..=LOCAL_HELPER_STARTUP_ATTEMPTS {
        match wait_for_checkpoint_slot(helper, minimum_slot).await {
            Ok(fork_choice) => {
                return Ok((fork_choice, helper_restarted));
            }
            Err(error)
                if attempt < LOCAL_HELPER_STARTUP_ATTEMPTS
                    && helper_startup_error_is_retryable(&error) =>
            {
                eprintln!(
                    "Restarting {LOCAL_HELPER_KIND} after finalized checkpoint wait failure on attempt {attempt}: {error}"
                );
                let (restarted_helper, _source_genesis_validator_entries) =
                    start_local_lean_spec_helper_with_genesis_metadata(helper_config).await?;
                *helper = restarted_helper;
                last_error = Some(error);
                helper_restarted = true;
                sleep(Duration::from_secs(LOCAL_HELPER_RETRY_DELAY_SECS)).await;
            }
            Err(error) => return Err(error),
        }
    }

    Err(last_error.unwrap_or_else(|| {
        format!(
            "{LOCAL_HELPER_KIND} never reached finalized slot {} after {} attempts",
            minimum_slot, LOCAL_HELPER_STARTUP_ATTEMPTS
        )
    }))
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
        "Client under test {} never reached a non-genesis justified checkpoint (last observed slot: {}, last error: {})",
        client.kind,
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

async fn wait_for_checkpoint_sync_client_post_genesis(client: &Client) -> Result<(), String> {
    let http = http_client();
    let url = lean_api_url(client, "/lean/v0/fork_choice");
    let mut last_error = String::new();
    let mut last_observed_state = None;

    for _attempt in 0..CHECKPOINT_SYNC_CLIENT_READY_TIMEOUT_SECS {
        match http.get(&url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    match response.json::<CheckpointSyncForkChoiceSnapshot>().await {
                        Ok(fork_choice) => {
                            let max_node_slot = fork_choice
                                .nodes
                                .iter()
                                .map(|node| node.slot)
                                .max()
                                .unwrap_or(0);
                            last_observed_state = Some(format!(
                                "justified_slot={}, finalized_slot={}, max_node_slot={}",
                                fork_choice.justified.slot,
                                fork_choice.finalized.slot,
                                max_node_slot
                            ));
                            if fork_choice.justified.slot > 0
                                || fork_choice.finalized.slot > 0
                                || max_node_slot > 0
                            {
                                return Ok(());
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
        "Client under test {} never exposed a post-genesis checkpoint-sync forkchoice state (last observed state: {}, last error: {})",
        client.kind,
        last_observed_state.unwrap_or_else(|| "none".to_string()),
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
    let url = helper.checkpoint_sync_url();
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

async fn wait_for_helper_to_reach_post_genesis(
    helper: &mut RunningLocalLeanSpecHelper,
) -> Result<(), String> {
    let mut last_error = String::new();

    for _attempt in 0..MESH_HELPER_READY_TIMEOUT_SECS {
        match wait_for_helper_to_reach_post_genesis_once(helper).await {
            Ok(true) => return Ok(()),
            Ok(false) => {
                last_error = format!(
                    "{} `{}` is still reporting a pre-genesis forkchoice",
                    LOCAL_HELPER_KIND, helper.node_id
                );
            }
            Err(err) => last_error = err,
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "{LOCAL_HELPER_KIND} `{}` never exposed a post-genesis forkchoice state (last error: {})",
        helper.node_id,
        if last_error.is_empty() {
            "none".to_string()
        } else {
            last_error
        }
    ))
}

async fn wait_for_mesh_helpers_to_reach_post_genesis(
    helpers: &mut [RunningLocalLeanSpecHelper],
) -> Result<(), String> {
    for helper in helpers {
        wait_for_helper_to_reach_post_genesis(helper).await?;
    }

    Ok(())
}

async fn wait_for_any_mesh_helper_to_reach_post_genesis(
    helpers: &mut [RunningLocalLeanSpecHelper],
) -> Result<(), String> {
    if helpers.is_empty() {
        return Ok(());
    }

    let mut last_errors = Vec::new();

    for _attempt in 0..MESH_HELPER_READY_TIMEOUT_SECS {
        last_errors.clear();

        for helper in helpers.iter_mut() {
            match wait_for_helper_to_reach_post_genesis_once(helper).await {
                Ok(true) => return Ok(()),
                Ok(false) => last_errors.push(format!(
                    "{} `{}` is still reporting a pre-genesis forkchoice",
                    LOCAL_HELPER_KIND, helper.node_id
                )),
                Err(err) => last_errors.push(err),
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "No auxiliary {LOCAL_HELPER_KIND} reached a post-genesis forkchoice state within {} seconds: {}",
        MESH_HELPER_READY_TIMEOUT_SECS,
        last_errors.join(" | ")
    ))
}

async fn wait_for_helper_to_reach_post_genesis_once(
    helper: &mut RunningLocalLeanSpecHelper,
) -> Result<bool, String> {
    let http = http_client();
    let url = helper.fork_choice_url();

    helper.ensure_running()?;
    let response = http
        .get(&url)
        .send()
        .await
        .map_err(|err| format!("error sending request for url ({url}): {err}"))?;
    let status = response.status();
    if !status.is_success() {
        return Err(format!("received HTTP {status} from {url}"));
    }

    let fork_choice = response
        .json::<CheckpointSyncForkChoiceSnapshot>()
        .await
        .map_err(|err| format!("Unable to decode fork_choice response from {url}: {err}"))?;
    let max_node_slot = fork_choice
        .nodes
        .iter()
        .map(|node| node.slot)
        .max()
        .unwrap_or(0);

    Ok(fork_choice.justified.slot > 0 || fork_choice.finalized.slot > 0 || max_node_slot > 0)
}

pub(crate) fn http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("Unable to build HTTP client")
}

pub(crate) async fn load_response_with_retry(
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

async fn load_helper_genesis_metadata(
    helper: &mut RunningLocalLeanSpecHelper,
) -> Result<HelperGenesisMetadata, String> {
    let http = http_client();
    let url = helper.metadata_url();
    let mut last_error = String::new();
    let mut last_observed_genesis_time = None;

    for _attempt in 0..LOCAL_HELPER_METADATA_TIMEOUT_SECS {
        helper.ensure_running()?;
        match http.get(&url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    match response.json::<HelperGenesisMetadata>().await {
                        Ok(metadata) => {
                            last_observed_genesis_time = Some(metadata.genesis_time);
                            if metadata.genesis_time == helper.expected_genesis_time {
                                return Ok(metadata);
                            }

                            last_error = format!(
                                "observed helper genesis_time {} while waiting for {}",
                                metadata.genesis_time, helper.expected_genesis_time
                            );
                        }
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
        "Unable to load helper genesis metadata from {url} for genesis_time {} (last observed genesis_time: {}, last error: {})",
        helper.expected_genesis_time,
        last_observed_genesis_time
            .map(|genesis_time| genesis_time.to_string())
            .unwrap_or_else(|| "none".to_string()),
        if last_error.is_empty() {
            "none".to_string()
        } else {
            last_error
        }
    ))
}

fn helper_startup_error_is_retryable(error: &str) -> bool {
    error.contains("exited before the test completed")
        || error.contains("SIGSEGV")
        || error.contains("signal: 11")
        || error.contains("SIGABRT")
        || error.contains("signal: 6")
        || error.contains("SIGFPE")
        || error.contains("signal: 8")
        || error.contains("exit status: 134")
        || error.contains("exit status: 136")
}

async fn start_local_lean_spec_helper_with_genesis_metadata(
    helper_config: &LocalLeanSpecHelperConfig,
) -> Result<(RunningLocalLeanSpecHelper, Vec<HelperGenesisValidatorEntry>), String> {
    let mut last_error = None;

    for attempt in 1..=LOCAL_HELPER_STARTUP_ATTEMPTS {
        let mut helper = start_local_lean_spec_helper(helper_config);
        match load_helper_genesis_metadata(&mut helper).await {
            Ok(metadata) => {
                helper.bootnode_enr = metadata.bootnode_enr.clone();
                helper.bootnode_multiaddr = metadata.bootnode_multiaddr.clone();
                return Ok((helper, metadata.genesis_validator_entries));
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
                sleep(Duration::from_secs(LOCAL_HELPER_RETRY_DELAY_SECS)).await;
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

async fn start_mesh_helpers(
    source_helper: &RunningLocalLeanSpecHelper,
    genesis_time: u64,
    mesh_peer_count: usize,
    helper_fork_digest: String,
) -> Result<Vec<RunningLocalLeanSpecHelper>, String> {
    let mut mesh_helpers = Vec::with_capacity(mesh_peer_count);
    let source_bootnode = source_helper.bootnode_multiaddr();

    for mesh_index in 1..=mesh_peer_count {
        let helper_config = LocalLeanSpecHelperConfig::mesh_peer(
            mesh_index,
            genesis_time,
            source_bootnode.clone(),
            helper_fork_digest.clone(),
        );
        let (helper, _source_genesis_validator_entries) =
            start_local_lean_spec_helper_with_genesis_metadata(&helper_config).await?;
        mesh_helpers.push(helper);
    }

    Ok(mesh_helpers)
}

fn start_local_lean_spec_helper(
    helper_config: &LocalLeanSpecHelperConfig,
) -> RunningLocalLeanSpecHelper {
    let advertise_ip = simulator_container_ip();
    let asset_root = local_helper_runtime_asset_root(&helper_config.node_id);
    let mut command = Command::new(LOCAL_HELPER_ENTRYPOINT);
    command.stdout(Stdio::inherit());
    command.stderr(Stdio::inherit());

    for (key, value) in local_lean_spec_helper_environment(helper_config) {
        command.env(key, value);
    }
    command.env(
        LEAN_HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE,
        advertise_ip.to_string(),
    );
    command.env(LEAN_RUNTIME_ASSET_ROOT_ENVIRONMENT_VARIABLE, &asset_root);

    let child = command.spawn().unwrap_or_else(|err| {
        panic!(
            "Unable to start local LeanSpec helper from {}: {err}",
            LOCAL_HELPER_ENTRYPOINT
        )
    });

    RunningLocalLeanSpecHelper {
        child,
        ip: advertise_ip,
        expected_genesis_time: helper_config.genesis_time,
        node_id: helper_config.node_id.clone(),
        asset_root,
        p2p_port: helper_config.p2p_port,
        api_port: helper_config.api_port,
        metadata_port: helper_config.metadata_port,
        bootnode_enr: None,
        bootnode_multiaddr: None,
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::{Read, Write};
    use std::net::{Ipv4Addr, TcpListener};
    use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
    use std::sync::Arc;
    use std::thread;

    fn start_metadata_server(
        genesis_time: Arc<AtomicU64>,
        stop: Arc<AtomicBool>,
    ) -> thread::JoinHandle<()> {
        thread::spawn(move || {
            let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, DEFAULT_HELPER_METADATA_PORT))
                .expect("test metadata server should bind");
            listener
                .set_nonblocking(true)
                .expect("test metadata server should be nonblocking");

            while !stop.load(Ordering::SeqCst) {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        let mut request_buffer = [0_u8; 1024];
                        let _ = stream.read(&mut request_buffer);

                        let body = format!(
                            "{{\"genesis_time\":{},\"genesis_validator_entries\":[{{\"attestation_public_key\":\"0xabc\",\"proposal_public_key\":null}}]}}",
                            genesis_time.load(Ordering::SeqCst)
                        );
                        let response = format!(
                            "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                            body.len(),
                            body
                        );
                        stream
                            .write_all(response.as_bytes())
                            .expect("test metadata server should write response");
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(50));
                    }
                    Err(error) => panic!("test metadata server accept failed: {error}"),
                }
            }
        })
    }

    #[tokio::test]
    async fn load_helper_genesis_metadata_waits_for_expected_genesis_time() {
        let observed_genesis_time = Arc::new(AtomicU64::new(111));
        let stop_server = Arc::new(AtomicBool::new(false));
        let server_thread =
            start_metadata_server(observed_genesis_time.clone(), stop_server.clone());
        let mut helper = RunningLocalLeanSpecHelper {
            child: Command::new("sleep")
                .arg("30")
                .spawn()
                .expect("sleep should spawn for helper test"),
            ip: IpAddr::V4(Ipv4Addr::LOCALHOST),
            expected_genesis_time: 222,
            node_id: "test-helper".to_string(),
            asset_root: env::temp_dir().join("test-helper-assets"),
            p2p_port: DEFAULT_HELPER_P2P_PORT,
            api_port: DEFAULT_HELPER_API_PORT,
            metadata_port: DEFAULT_HELPER_METADATA_PORT,
            bootnode_enr: None,
            bootnode_multiaddr: None,
        };

        let updater = {
            let observed_genesis_time = observed_genesis_time.clone();
            thread::spawn(move || {
                thread::sleep(Duration::from_millis(1100));
                observed_genesis_time.store(222, Ordering::SeqCst);
            })
        };

        let metadata = load_helper_genesis_metadata(&mut helper)
            .await
            .expect("metadata should eventually match the expected genesis time");

        assert_eq!(metadata.genesis_time, 222);
        assert_eq!(metadata.genesis_validator_entries.len(), 1);
        assert_eq!(
            metadata.genesis_validator_entries[0].attestation_public_key,
            "0xabc"
        );

        stop_server.store(true, Ordering::SeqCst);
        updater.join().expect("metadata updater should finish");
        server_thread
            .join()
            .expect("test metadata server should shut down cleanly");
    }

    #[test]
    fn helper_startup_error_is_retryable_for_sigabrt() {
        assert!(helper_startup_error_is_retryable(
            "lean-spec-local-helper exited before the test completed with status signal: 6 (SIGABRT) (core dumped)"
        ));
    }

    #[test]
    fn helper_startup_error_is_retryable_for_sigsegv() {
        assert!(helper_startup_error_is_retryable(
            "lean-spec-local-helper exited before the test completed with status signal: 11 (SIGSEGV) (core dumped)"
        ));
    }

    #[test]
    fn helper_startup_error_is_retryable_for_sigfpe() {
        assert!(helper_startup_error_is_retryable(
            "lean-spec-local-helper exited before the test completed with status signal: 8 (SIGFPE) (core dumped)"
        ));
    }

    #[test]
    fn adjusted_genesis_time_for_helper_mesh_preserves_non_mesh_requests() {
        assert_eq!(adjusted_genesis_time_for_helper_mesh(123, 1, false), 123);
    }

    #[test]
    fn adjusted_genesis_time_for_helper_mesh_pushes_stale_mesh_genesis_forward() {
        let adjusted = adjusted_genesis_time_for_helper_mesh(0, 3, true);
        let minimum_expected =
            default_genesis_time() + (2 * MESH_HELPER_GENESIS_BUFFER_PER_PEER_SECS);
        assert!(adjusted >= minimum_expected);
    }

    #[test]
    fn adjusted_genesis_time_for_helper_mesh_preserves_future_requests() {
        let requested = default_genesis_time() + 100;
        assert_eq!(
            adjusted_genesis_time_for_helper_mesh(requested, 3, true),
            requested
        );
    }
}
