//! Shared Lean scenario test helpers, including post-genesis sync setup.

use std::collections::HashMap;
use std::env;
use std::fs;
use std::net::IpAddr;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use crate::utils::util::{
    bootnode_enr_for_client, client_uses_enr_bootnodes, current_unix_time, default_genesis_time,
    fork_choice_head_slot, http_client, lean_api_url, lean_environment, panic_payload_to_string,
    prepare_client_runtime_files, selected_lean_devnet, simulator_container_ip, CheckpointResponse,
    ClientUnderTestRole, ForkChoiceResponse, ForkChoiceSnapshot, LeanDevnet,
    DEVNET4_HELPER_GOSSIP_FORK_DIGEST, LEAN_HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE,
    LEAN_HELPER_API_PORT_ENVIRONMENT_VARIABLE, LEAN_HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE,
    LEAN_HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE,
    LEAN_HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE, LEAN_HELPER_P2P_PORT_ENVIRONMENT_VARIABLE,
};
use hivesim::types::ClientDefinition;
use hivesim::{Client, Test};
use serde::Deserialize;
use tokio::time::{sleep, timeout};

const CHECKPOINT_SYNC_URL_ENVIRONMENT_VARIABLE: &str = "HIVE_CHECKPOINT_SYNC_URL";
const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const LEAN_GENESIS_VALIDATORS_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_VALIDATORS";
const LEAN_GENESIS_VALIDATOR_ENTRIES_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";
const CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const DISABLE_VALIDATOR_SERVICE_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_DISABLE_VALIDATOR_SERVICE";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_COUNT";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const LEAN_RUNTIME_ASSET_ROOT_ENVIRONMENT_VARIABLE: &str = "LEAN_RUNTIME_ASSET_ROOT";
const LEAN_SPEC_SOURCE_NODE_ID: &str = "lean_spec_0";
const LEAN_SPEC_SOURCE_VALIDATORS: &str = "0,1,2";
/// Helper validator subset used by tests where the client-under-test owns V0.
pub(crate) const LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0: &str = "1,2,3";
const LEAN_SPEC_SOURCE_PEER_ID: &str = "16Uiu2HAmHzBkRq62mG95vsjKMuYQBezZCtjPXYWUoyVxMxi71aB3";
const DEFAULT_HELPER_GOSSIP_FORK_DIGEST: &str = "devnet0";
const DEFAULT_HELPER_P2P_PORT: u16 = 9001;
const DEFAULT_HELPER_API_PORT: u16 = 5052;
const DEFAULT_HELPER_METADATA_PORT: u16 = 5053;
const MESH_HELPER_GENESIS_BUFFER_PER_PEER_SECS: u64 = 10;
const MESH_HELPER_SOURCE_STARTUP_CUSHION_SECS: u64 = 60;
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
const OPTIONAL_MESH_HELPER_READY_GRACE_SECS: u64 = 15;
const LIVE_HELPER_FORK_CHOICE_RETRY_ATTEMPTS: u64 = 10;
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
    genesis_time: u64,
    genesis_validator_entries: Vec<HelperGenesisValidatorEntry>,
    #[serde(default)]
    bootnode_qlean_enr: Option<String>,
    bootnode_enr: Option<String>,
    bootnode_multiaddr: Option<String>,
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
    pub split_helper_validators_across_mesh: bool,
    pub helper_peer_count: usize,
    pub helper_fork_digest_profile: HelperGossipForkDigestProfile,
}

pub(crate) struct PostGenesisSyncContext {
    _helpers: RunningLocalLeanSpecHelperGroup,
    pub client_under_test: Client,
    pub source_fork_choice: ForkChoiceSnapshot,
    pub client_checkpoint: Option<CheckpointResponse>,
}

pub(crate) struct CheckpointSyncHelperMesh {
    helpers: RunningLocalLeanSpecHelperGroup,
    source_fork_choice: ForkChoiceSnapshot,
    source_genesis_validator_entries: Vec<HelperGenesisValidatorEntry>,
    source_helper_config: LocalLeanSpecHelperConfig,
    genesis_time: u64,
}

impl PostGenesisSyncContext {
    pub(crate) async fn try_load_live_helper_fork_choice(
        &mut self,
    ) -> Result<ForkChoiceResponse, String> {
        self._helpers.load_live_fork_choice().await
    }

    pub(crate) async fn load_live_helper_fork_choice(&mut self) -> ForkChoiceResponse {
        self.try_load_live_helper_fork_choice()
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
    bootnode_qlean_enr: Option<String>,
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
    genesis_validator_count: u64,
    genesis_time: u64,
    bootnodes: Option<String>,
    is_aggregator: bool,
    disable_validator_service: bool,
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
        genesis_validator_count: u64,
    ) -> Self {
        Self {
            node_id: LEAN_SPEC_SOURCE_NODE_ID.to_string(),
            validator_indices: validator_indices
                .unwrap_or_else(|| LEAN_SPEC_SOURCE_VALIDATORS.to_string()),
            genesis_validator_count,
            genesis_time,
            bootnodes: None,
            is_aggregator: true,
            disable_validator_service: false,
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
        validator_indices: String,
        genesis_validator_count: u64,
        disable_validator_service: bool,
        gossip_fork_digest: String,
    ) -> Self {
        let mesh_index = mesh_index as u16;
        let is_aggregator = !disable_validator_service && !validator_indices.is_empty();
        Self {
            node_id: format!("lean_spec_mesh_{mesh_index}"),
            validator_indices,
            genesis_validator_count,
            genesis_time,
            bootnodes: Some(bootnode),
            is_aggregator,
            disable_validator_service,
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

    fn checkpoint_sync_block_url(&self) -> String {
        format!(
            "http://{}:{}/lean/v0/blocks/finalized",
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
        if client_uses_enr_bootnodes(client_type) {
            if let Some(enr) = bootnode_enr_for_client(
                client_type,
                self.bootnode_enr.as_deref(),
                self.bootnode_qlean_enr.as_deref(),
            ) {
                return enr.to_string();
            }

            return self.bootnode_multiaddr();
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
        self.child.kill().ok();
        self.child.wait().ok();
        fs::remove_dir_all(&self.asset_root).ok();
    }
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
    let genesis_validator_count = validator_count_for_indices(LEAN_SPEC_SOURCE_VALIDATORS);
    let source_helper_config = LocalLeanSpecHelperConfig::source(
        genesis_time,
        helper_fork_digest,
        None,
        genesis_validator_count,
    );
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

fn adjusted_genesis_time_for_helper_mesh(
    requested_genesis_time: u64,
    helper_peer_count: usize,
    connect_client_to_lean_spec_mesh: bool,
) -> u64 {
    let helper_startup_buffer = helper_mesh_pre_genesis_launch_buffer_secs(
        helper_peer_count,
        connect_client_to_lean_spec_mesh,
    );
    if helper_startup_buffer == 0 {
        return requested_genesis_time;
    }

    // Mesh helpers don't backfill the earliest chain history on startup so they need a fresh pre-genesis launch window to observe the canonical chain from
    // the beginning instead of joining after the source helper is already live
    requested_genesis_time.max(
        default_genesis_time() + helper_startup_buffer + MESH_HELPER_SOURCE_STARTUP_CUSHION_SECS,
    )
}

fn helper_mesh_pre_genesis_launch_buffer_secs(
    helper_peer_count: usize,
    connect_client_to_lean_spec_mesh: bool,
) -> u64 {
    if !connect_client_to_lean_spec_mesh || helper_peer_count <= 1 {
        0
    } else {
        ((helper_peer_count - 1) as u64) * MESH_HELPER_GENESIS_BUFFER_PER_PEER_SECS
    }
}

fn helper_mesh_genesis_has_launch_window(
    genesis_time: u64,
    helper_peer_count: usize,
    connect_client_to_lean_spec_mesh: bool,
) -> bool {
    let helper_startup_buffer = helper_mesh_pre_genesis_launch_buffer_secs(
        helper_peer_count,
        connect_client_to_lean_spec_mesh,
    );
    if helper_startup_buffer == 0 {
        return true;
    }

    genesis_time >= current_unix_time() + helper_startup_buffer
}

async fn refresh_source_helper_for_mesh_launch_window(
    mut source_helper: RunningLocalLeanSpecHelper,
    mut source_genesis_validator_entries: Vec<HelperGenesisValidatorEntry>,
    source_helper_config: &mut LocalLeanSpecHelperConfig,
    helper_peer_count: usize,
    connect_client_to_lean_spec_mesh: bool,
) -> Result<(RunningLocalLeanSpecHelper, Vec<HelperGenesisValidatorEntry>), String> {
    let mut refresh_attempt = 0;
    while !helper_mesh_genesis_has_launch_window(
        source_helper_config.genesis_time,
        helper_peer_count,
        connect_client_to_lean_spec_mesh,
    ) {
        refresh_attempt += 1;
        if refresh_attempt > LOCAL_HELPER_STARTUP_ATTEMPTS {
            return Err(format!(
                "Unable to refresh {LOCAL_HELPER_KIND} with a genesis_time that leaves a mesh pre-genesis launch window after {} attempts",
                LOCAL_HELPER_STARTUP_ATTEMPTS
            ));
        }

        let stale_genesis_time = source_helper_config.genesis_time;
        drop(source_helper);
        source_helper_config.genesis_time = adjusted_genesis_time_for_helper_mesh(
            default_genesis_time(),
            helper_peer_count,
            connect_client_to_lean_spec_mesh,
        );
        eprintln!(
            "Restarting {LOCAL_HELPER_KIND} with fresh genesis_time {} after startup consumed the mesh pre-genesis launch window for genesis_time {}",
            source_helper_config.genesis_time, stale_genesis_time
        );

        let refreshed_source =
            start_local_lean_spec_helper_with_genesis_metadata(source_helper_config).await?;
        source_helper = refreshed_source.0;
        source_genesis_validator_entries = refreshed_source.1;
    }

    Ok((source_helper, source_genesis_validator_entries))
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

fn helper_mesh_validator_assignments(
    test_data: &PostGenesisSyncTestData,
    helper_peer_count: usize,
) -> Vec<String> {
    let helper_peer_count = helper_peer_count.max(1);
    let source_validator_indices = source_helper_validator_indices(test_data);

    if test_data.split_helper_validators_across_mesh {
        return split_validator_indices_across_helpers(
            &source_validator_indices,
            helper_peer_count,
        );
    }

    if should_start_passive_validator_mesh(test_data, helper_peer_count) {
        return vec![source_validator_indices; helper_peer_count];
    }

    let mut assignments = vec![String::new(); helper_peer_count];
    assignments[0] = source_validator_indices;
    assignments
}

fn source_helper_validator_indices(test_data: &PostGenesisSyncTestData) -> String {
    test_data
        .source_helper_validator_indices
        .clone()
        .unwrap_or_else(|| LEAN_SPEC_SOURCE_VALIDATORS.to_string())
}

fn helper_genesis_validator_count(test_data: &PostGenesisSyncTestData) -> u64 {
    validator_count_for_indices(&source_helper_validator_indices(test_data))
}

fn validator_count_for_indices(validator_indices: &str) -> u64 {
    validator_indices
        .split(',')
        .map(str::trim)
        .filter(|validator_index| !validator_index.is_empty())
        .map(|validator_index| {
            validator_index.parse::<u64>().unwrap_or_else(|err| {
                panic!("invalid Lean validator index {validator_index:?}: {err}")
            })
        })
        .max()
        .map(|max_validator_index| max_validator_index + 1)
        .unwrap_or(0)
}

fn split_validator_indices_across_helpers(
    validator_indices: &str,
    helper_peer_count: usize,
) -> Vec<String> {
    let helper_peer_count = helper_peer_count.max(1);
    let validator_indices = validator_indices
        .split(',')
        .map(str::trim)
        .filter(|validator_index| !validator_index.is_empty())
        .map(str::to_string)
        .collect::<Vec<_>>();
    let validator_count = validator_indices.len();
    let base_assignment_size = validator_count / helper_peer_count;
    let extra_assignments = validator_count % helper_peer_count;
    let mut next_validator = 0;

    (0..helper_peer_count)
        .map(|helper_index| {
            let assignment_size =
                base_assignment_size + usize::from(helper_index < extra_assignments);
            let assignment =
                validator_indices[next_validator..next_validator + assignment_size].join(",");
            next_validator += assignment_size;
            assignment
        })
        .collect()
}

fn should_start_passive_validator_mesh(
    test_data: &PostGenesisSyncTestData,
    helper_peer_count: usize,
) -> bool {
    !test_data.split_helper_validators_across_mesh
        && !test_data.use_checkpoint_sync
        && test_data.connect_client_to_lean_spec_mesh
        && helper_peer_count > 1
}

pub(crate) async fn start_checkpoint_sync_helper_mesh(
    test_data: &PostGenesisSyncTestData,
) -> CheckpointSyncHelperMesh {
    assert!(
        test_data.use_checkpoint_sync,
        "checkpoint-sync helper mesh setup requires checkpoint sync test data"
    );

    let helper_peer_count = test_data.helper_peer_count.max(1);
    let genesis_time = adjusted_genesis_time_for_helper_mesh(
        test_data.genesis_time,
        helper_peer_count,
        test_data.connect_client_to_lean_spec_mesh,
    );
    let helper_fork_digest = helper_gossip_fork_digest(test_data.helper_fork_digest_profile);
    let genesis_validator_count = helper_genesis_validator_count(test_data);
    let helper_validator_assignments =
        helper_mesh_validator_assignments(test_data, helper_peer_count);
    let mut source_helper_config = LocalLeanSpecHelperConfig::source(
        genesis_time,
        helper_fork_digest.clone(),
        Some(helper_validator_assignments[0].clone()),
        genesis_validator_count,
    );
    let (source_helper, source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(&source_helper_config)
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to load finalized genesis validators from {LOCAL_HELPER_KIND}: {err}"
                )
            });
    let (source_helper, source_genesis_validator_entries) =
        refresh_source_helper_for_mesh_launch_window(
            source_helper,
            source_genesis_validator_entries,
            &mut source_helper_config,
            helper_peer_count,
            test_data.connect_client_to_lean_spec_mesh,
        )
        .await
        .unwrap_or_else(|err| {
            panic!(
                "Unable to refresh {LOCAL_HELPER_KIND} genesis before helper mesh startup: {err}"
            )
        });
    let genesis_time = source_helper_config.genesis_time;
    let mesh_peers = if test_data.connect_client_to_lean_spec_mesh && helper_peer_count > 1 {
        start_mesh_helpers(
            &source_helper,
            genesis_time,
            &helper_validator_assignments[1..],
            genesis_validator_count,
            false,
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

    let (mut source_fork_choice, mut source_helper_restarted) =
        match wait_for_checkpoint_slot_with_retry(
            &mut helpers.source,
            minimum_source_checkpoint_slot(test_data),
            &source_helper_config,
        )
        .await
        {
            Ok(source_fork_choice) => source_fork_choice,
            Err(err) => panic!("{err}"),
        };

    source_helper_restarted |= ensure_checkpoint_sync_source_ready(
        &mut helpers.source,
        &mut source_fork_choice,
        &source_helper_config,
        minimum_source_checkpoint_slot(test_data),
    )
    .await
    .unwrap_or_else(|err| panic!("{err}"));

    if test_data.connect_client_to_lean_spec_mesh && helper_peer_count > 1 {
        if source_helper_restarted {
            helpers.mesh_peers = start_mesh_helpers(
                &helpers.source,
                genesis_time,
                &helper_validator_assignments[1..],
                genesis_validator_count,
                false,
                helper_fork_digest.clone(),
            )
            .await
            .unwrap_or_else(|err| panic!("{err}"));
        }

        if let Err(err) =
            wait_briefly_for_any_mesh_helper_to_reach_post_genesis(&mut helpers.mesh_peers).await
        {
            eprintln!("Continuing checkpoint-sync test despite auxiliary helper lag: {err}");
        }
    }

    CheckpointSyncHelperMesh {
        helpers,
        source_fork_choice,
        source_genesis_validator_entries,
        source_helper_config,
        genesis_time,
    }
}

pub(crate) async fn start_checkpoint_sync_client_context(
    test: &Test,
    test_data: &PostGenesisSyncTestData,
    mut helper_mesh: CheckpointSyncHelperMesh,
) -> PostGenesisSyncContext {
    assert!(
        test_data.use_checkpoint_sync,
        "checkpoint-sync client startup requires checkpoint sync test data"
    );

    let checkpoint_sync_client_environment = client_under_test_environment(
        &helper_mesh.helpers,
        &test_data.client_under_test.name,
        helper_mesh.genesis_time,
        &helper_mesh.source_genesis_validator_entries,
        test_data.use_checkpoint_sync,
        test_data.connect_client_to_lean_spec_mesh,
        test_data.client_role,
    );
    let client_under_test =
        start_checkpoint_sync_client_under_test_with_retry(CheckpointSyncClientStart {
            test,
            helper: &mut helper_mesh.helpers.source,
            source_fork_choice: &mut helper_mesh.source_fork_choice,
            helper_config: helper_mesh.source_helper_config.clone(),
            client_type: test_data.client_under_test.name.clone(),
            environment: checkpoint_sync_client_environment,
            minimum_slot: minimum_source_checkpoint_slot(test_data),
        })
        .await;

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
        _helpers: helper_mesh.helpers,
        client_under_test,
        source_fork_choice: helper_mesh.source_fork_choice,
        client_checkpoint,
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
    let passive_validator_mesh = should_start_passive_validator_mesh(test_data, helper_peer_count);
    let genesis_validator_count = helper_genesis_validator_count(test_data);
    let helper_validator_assignments =
        helper_mesh_validator_assignments(test_data, helper_peer_count);
    let mut source_helper_config = LocalLeanSpecHelperConfig::source(
        genesis_time,
        helper_fork_digest.clone(),
        Some(helper_validator_assignments[0].clone()),
        genesis_validator_count,
    );
    let (source_helper, source_genesis_validator_entries) =
        start_local_lean_spec_helper_with_genesis_metadata(&source_helper_config)
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Unable to load finalized genesis validators from {LOCAL_HELPER_KIND}: {err}"
                )
            });
    let (source_helper, source_genesis_validator_entries) =
        refresh_source_helper_for_mesh_launch_window(
            source_helper,
            source_genesis_validator_entries,
            &mut source_helper_config,
            helper_peer_count,
            test_data.connect_client_to_lean_spec_mesh,
        )
        .await
        .unwrap_or_else(|err| {
            panic!(
                "Unable to refresh {LOCAL_HELPER_KIND} genesis before post-genesis helper mesh startup: {err}"
            )
        });
    let genesis_time = source_helper_config.genesis_time;
    let mesh_peers = if test_data.connect_client_to_lean_spec_mesh && helper_peer_count > 1 {
        start_mesh_helpers(
            &source_helper,
            genesis_time,
            &helper_validator_assignments[1..],
            genesis_validator_count,
            passive_validator_mesh,
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

    let (mut source_fork_choice, mut source_helper_restarted) = if should_start_client_early {
        match wait_for_checkpoint_slot(
            &mut helpers.source,
            minimum_source_checkpoint_slot(test_data),
        )
        .await
        {
            Ok(source_fork_choice) => (source_fork_choice, false),
            Err(error) => panic!(
                "{}",
                helper_failed_after_client_started_message(
                    &test_data.client_under_test.name,
                    "waiting for the source helper to reach the required finalized checkpoint",
                    &error,
                )
            ),
        }
    } else {
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
                    test.start_client_with_files(
                        test_data.client_under_test.name.clone(),
                        Some(initial_client_under_test_environment.clone()),
                        Some(files),
                    )
                    .await;
                }

                panic!("{err}");
            }
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

    if !test_data.use_checkpoint_sync && source_helper_restarted {
        if test_data.connect_client_to_lean_spec_mesh && helper_peer_count > 1 {
            eprintln!(
                "Restarting post-genesis helper mesh after {LOCAL_HELPER_KIND} was refreshed during source finalization wait"
            );
            helpers.mesh_peers = start_mesh_helpers(
                &helpers.source,
                genesis_time,
                &helper_validator_assignments[1..],
                genesis_validator_count,
                passive_validator_mesh,
                helper_fork_digest.clone(),
            )
            .await
            .unwrap_or_else(|err| panic!("{err}"));
        }

        if client_under_test.is_some() {
            panic!(
                "{}",
                helper_failed_after_client_started_message(
                    &test_data.client_under_test.name,
                    "refreshing the source helper during post-genesis setup",
                    "the source helper was restarted after the client under test had already started",
                )
            );
        }
    }

    if !test_data.use_checkpoint_sync && !helpers.mesh_peers.is_empty() {
        match wait_for_all_mesh_helpers_to_reach_post_genesis(&mut helpers.mesh_peers).await {
            Ok(()) => {}
            Err(error)
                if should_start_client_early && helper_startup_error_is_retryable(&error) =>
            {
                panic!(
                    "{}",
                    helper_failed_after_client_started_message(
                        &test_data.client_under_test.name,
                        "waiting for auxiliary helpers to reach post-genesis forkchoice",
                        &error,
                    )
                )
            }
            Err(error) => {
                eprintln!(
                    "Continuing post-genesis sync test despite auxiliary helper lag: {error}"
                );
            }
        }
    }

    if test_data.use_checkpoint_sync
        && test_data.connect_client_to_lean_spec_mesh
        && helper_peer_count > 1
    {
        if source_helper_restarted {
            helpers.mesh_peers = start_mesh_helpers(
                &helpers.source,
                genesis_time,
                &helper_validator_assignments[1..],
                genesis_validator_count,
                false,
                helper_fork_digest.clone(),
            )
            .await
            .unwrap_or_else(|err| panic!("{err}"));
        }

        if let Err(err) =
            wait_briefly_for_any_mesh_helper_to_reach_post_genesis(&mut helpers.mesh_peers).await
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

fn helper_failed_after_client_started_message(
    client_name: &str,
    phase: &str,
    reason: &str,
) -> String {
    format!(
        "{LOCAL_HELPER_KIND} failed after client under test {client_name} was started while {phase}; test result is indeterminate and should not be interpreted as a client failure. Helper failure: {reason}"
    )
}

fn lean_spec_environment(
    node_id: &str,
    validator_indices: &str,
    genesis_time: u64,
    bootnodes: Option<String>,
    is_aggregator: bool,
    disable_validator_service: bool,
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

    if disable_validator_service {
        environment.insert(
            DISABLE_VALIDATOR_SERVICE_ENVIRONMENT_VARIABLE.to_string(),
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
        helper_config.disable_validator_service,
    );
    environment.insert(
        LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE.to_string(),
        helper_config.genesis_validator_count.to_string(),
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
    // Pin the libp2p peer id to the value `compute_client_peer_id` derives,
    // so MockNode dials the client at its actual peer id instead of an
    // unrelated one (which surfaces as `Outbound failure: DialFailure`).
    environment.insert(
        CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(),
        crate::utils::libp2p_mock::deterministic_client_private_key_hex(client_type),
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

    client_role.apply_to_environment(&mut environment);

    environment
}

fn minimum_source_checkpoint_slot(test_data: &PostGenesisSyncTestData) -> u64 {
    if test_data.use_checkpoint_sync {
        MIN_FINALIZED_SLOT_FOR_CHECKPOINT_SYNC
    } else {
        1
    }
}

fn local_helper_runtime_asset_root(node_id: &str) -> PathBuf {
    let timestamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    env::temp_dir().join(format!("lean-spec-helper-{node_id}-{timestamp}"))
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
            handle.await.ok();
            Err(format!(
                "startup attempt exceeded {CLIENT_UNDER_TEST_STARTUP_ATTEMPT_TIMEOUT_SECS} seconds"
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
                    "Retrying client-under-test startup for {client_type} after attempt {attempt} failed: {message}"
                );
                last_error = Some(message);
                sleep(Duration::from_secs(1)).await;
            }
            Err(message) => {
                panic!(
                    "Unable to start client under test {client_type} after {CLIENT_UNDER_TEST_STARTUP_ATTEMPTS} attempts: {message}"
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
    let mut client_was_started = false;

    for attempt in 1..=CLIENT_UNDER_TEST_STARTUP_ATTEMPTS {
        if client_was_started {
            wait_for_checkpoint_sync_state_ready(params.helper)
                .await
                .unwrap_or_else(|err| {
                    panic!(
                        "{}",
                        helper_failed_after_client_started_message(
                            &params.client_type,
                            "checking checkpoint-sync source readiness before a retry",
                            &err,
                        )
                    )
                });
        } else {
            ensure_checkpoint_sync_source_ready(
                params.helper,
                params.source_fork_choice,
                &params.helper_config,
                params.minimum_slot,
            )
            .await
            .unwrap_or_else(|err| {
                panic!(
                    "Checkpoint-sync source state endpoint was not ready for {} before initial client startup attempt {}: {}",
                    params.client_type, attempt, err
                )
            });
        }
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
                    client_was_started = true;
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
            "{LOCAL_HELPER_KIND} never reached finalized slot {minimum_slot} after {LOCAL_HELPER_STARTUP_ATTEMPTS} attempts"
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
    let state_url = helper.checkpoint_sync_url();
    let block_url = helper.checkpoint_sync_block_url();
    let mut last_error = String::new();
    let mut consecutive_successes = 0;

    for _attempt in 0..LOCAL_HELPER_STARTUP_TIMEOUT_SECS {
        helper.ensure_running()?;
        let state_result = http
            .get(&state_url)
            .header(reqwest::header::ACCEPT, SSZ_CONTENT_TYPE)
            .send()
            .await;
        let block_result = http
            .get(&block_url)
            .header(reqwest::header::ACCEPT, SSZ_CONTENT_TYPE)
            .send()
            .await;

        let state_ready = match state_result {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {state_url}");
                }
                status.is_success()
            }
            Err(err) => {
                last_error = format!("error sending request for url ({state_url}): {err}");
                false
            }
        };

        let block_ready = match block_result {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {block_url}");
                }
                status.is_success()
            }
            Err(err) => {
                last_error = format!("error sending request for url ({block_url}): {err}");
                false
            }
        };

        if state_ready && block_ready {
            consecutive_successes += 1;
            if consecutive_successes >= 3 {
                return Ok(());
            }
        } else {
            consecutive_successes = 0;
            if !state_ready && last_error.is_empty() {
                last_error = format!("{state_url} was not ready");
            }
        }

        sleep(Duration::from_secs(1)).await;
    }

    Err(format!(
        "Checkpoint-sync source state/block endpoints never became ready at {state_url} and {block_url}: {last_error}"
    ))
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

async fn wait_for_all_mesh_helpers_to_reach_post_genesis(
    helpers: &mut [RunningLocalLeanSpecHelper],
) -> Result<(), String> {
    if helpers.is_empty() {
        return Ok(());
    }

    let mut helper_ready = vec![false; helpers.len()];
    let mut last_errors = Vec::new();

    for _attempt in 0..MESH_HELPER_READY_TIMEOUT_SECS {
        last_errors.clear();

        for (index, helper) in helpers.iter_mut().enumerate() {
            if helper_ready[index] {
                continue;
            }

            match wait_for_helper_to_reach_post_genesis_once(helper).await {
                Ok(true) => helper_ready[index] = true,
                Ok(false) => last_errors.push(format!(
                    "{} `{}` is still reporting a pre-genesis forkchoice",
                    LOCAL_HELPER_KIND, helper.node_id
                )),
                Err(err) => last_errors.push(err),
            }
        }

        if helper_ready.iter().all(|ready| *ready) {
            return Ok(());
        }

        sleep(Duration::from_secs(1)).await;
    }

    let ready_count = helper_ready.iter().filter(|ready| **ready).count();
    Err(format!(
        "Only {ready_count}/{} auxiliary {LOCAL_HELPER_KIND} instances reached a post-genesis forkchoice state within {} seconds: {}",
        helpers.len(),
        MESH_HELPER_READY_TIMEOUT_SECS,
        last_errors.join(" | ")
    ))
}

async fn wait_briefly_for_any_mesh_helper_to_reach_post_genesis(
    helpers: &mut [RunningLocalLeanSpecHelper],
) -> Result<(), String> {
    match timeout(
        Duration::from_secs(OPTIONAL_MESH_HELPER_READY_GRACE_SECS),
        wait_for_any_mesh_helper_to_reach_post_genesis(helpers),
    )
    .await
    {
        Ok(result) => result,
        Err(_) => Err(format!(
            "No auxiliary {LOCAL_HELPER_KIND} reached a post-genesis forkchoice state within {} seconds",
            OPTIONAL_MESH_HELPER_READY_GRACE_SECS
        )),
    }
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
        || error.contains("Unable to load helper genesis metadata")
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
                helper.bootnode_qlean_enr = metadata.bootnode_qlean_enr.clone();
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
            "Unable to start {LOCAL_HELPER_KIND} after {LOCAL_HELPER_STARTUP_ATTEMPTS} attempts"
        )
    }))
}

async fn start_mesh_helpers(
    source_helper: &RunningLocalLeanSpecHelper,
    genesis_time: u64,
    mesh_validator_indices: &[String],
    genesis_validator_count: u64,
    disable_validator_service: bool,
    helper_fork_digest: String,
) -> Result<Vec<RunningLocalLeanSpecHelper>, String> {
    let mesh_peer_count = mesh_validator_indices.len();
    let mut mesh_helpers = Vec::with_capacity(mesh_peer_count);
    let source_bootnode = source_helper.bootnode_multiaddr();

    for (mesh_index, validator_indices) in mesh_validator_indices.iter().enumerate() {
        let mesh_index = mesh_index + 1;
        let helper_config = LocalLeanSpecHelperConfig::mesh_peer(
            mesh_index,
            genesis_time,
            source_bootnode.clone(),
            validator_indices.clone(),
            genesis_validator_count,
            disable_validator_service,
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
        panic!("Unable to start local LeanSpec helper from {LOCAL_HELPER_ENTRYPOINT}: {err}")
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
        bootnode_qlean_enr: None,
        bootnode_enr: None,
        bootnode_multiaddr: None,
    }
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
                        let bytes_read = match stream.read(&mut request_buffer) {
                            Ok(bytes_read) => bytes_read,
                            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => 0,
                            Err(error) => {
                                panic!("test metadata server should read request: {error}")
                            }
                        };
                        if bytes_read == 0 {
                            thread::yield_now();
                        }

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
            bootnode_qlean_enr: None,
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
    fn helper_startup_error_is_retryable_for_metadata_timeout() {
        assert!(helper_startup_error_is_retryable(
            "Unable to load helper genesis metadata from http://127.0.0.1:5053/hive/genesis for genesis_time 1 (last observed genesis_time: none, last error: error sending request for url)"
        ));
    }

    #[test]
    fn helper_mesh_validator_assignments_repeat_source_for_passive_mesh() {
        let mut test_data = PostGenesisSyncTestData {
            client_under_test: ClientDefinition {
                name: "ream_devnet4".to_string(),
                version: "test".to_string(),
                meta: hivesim::types::ClientMetadata { roles: Vec::new() },
            },
            genesis_time: 1,
            wait_for_client_justified_checkpoint: false,
            use_checkpoint_sync: false,
            connect_client_to_lean_spec_mesh: true,
            client_role: ClientUnderTestRole::Validator,
            source_helper_validator_indices: None,
            split_helper_validators_across_mesh: false,
            helper_peer_count: 3,
            helper_fork_digest_profile: HelperGossipForkDigestProfile::SelectedDevnet,
        };

        assert_eq!(
            helper_mesh_validator_assignments(&test_data, 3),
            vec![LEAN_SPEC_SOURCE_VALIDATORS.to_string(); 3]
        );

        test_data.use_checkpoint_sync = true;
        assert_eq!(
            helper_mesh_validator_assignments(&test_data, 3),
            vec![
                LEAN_SPEC_SOURCE_VALIDATORS.to_string(),
                String::new(),
                String::new()
            ]
        );
    }

    #[test]
    fn helper_mesh_validator_assignments_split_validators_across_mesh() {
        let test_data = PostGenesisSyncTestData {
            client_under_test: ClientDefinition {
                name: "ream_devnet4".to_string(),
                version: "test".to_string(),
                meta: hivesim::types::ClientMetadata { roles: Vec::new() },
            },
            genesis_time: 1,
            wait_for_client_justified_checkpoint: false,
            use_checkpoint_sync: false,
            connect_client_to_lean_spec_mesh: true,
            client_role: ClientUnderTestRole::Validator,
            source_helper_validator_indices: None,
            split_helper_validators_across_mesh: true,
            helper_peer_count: 3,
            helper_fork_digest_profile: HelperGossipForkDigestProfile::SelectedDevnet,
        };

        assert_eq!(
            helper_mesh_validator_assignments(&test_data, 3),
            vec!["0".to_string(), "1".to_string(), "2".to_string()]
        );
        assert!(!should_start_passive_validator_mesh(&test_data, 3));
    }

    #[test]
    fn helper_mesh_validator_assignments_split_client_excluding_validators() {
        let test_data = PostGenesisSyncTestData {
            client_under_test: ClientDefinition {
                name: "ream_devnet4".to_string(),
                version: "test".to_string(),
                meta: hivesim::types::ClientMetadata { roles: Vec::new() },
            },
            genesis_time: 1,
            wait_for_client_justified_checkpoint: false,
            use_checkpoint_sync: false,
            connect_client_to_lean_spec_mesh: true,
            client_role: ClientUnderTestRole::Validator,
            source_helper_validator_indices: Some(
                LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
            ),
            split_helper_validators_across_mesh: true,
            helper_peer_count: 3,
            helper_fork_digest_profile: HelperGossipForkDigestProfile::SelectedDevnet,
        };

        assert_eq!(
            helper_mesh_validator_assignments(&test_data, 3),
            vec!["1".to_string(), "2".to_string(), "3".to_string()]
        );
        assert_eq!(helper_genesis_validator_count(&test_data), 4);
    }

    #[test]
    fn helper_mesh_validator_assignments_split_client_excluding_validators_across_two_helpers() {
        let test_data = PostGenesisSyncTestData {
            client_under_test: ClientDefinition {
                name: "ream_devnet4".to_string(),
                version: "test".to_string(),
                meta: hivesim::types::ClientMetadata { roles: Vec::new() },
            },
            genesis_time: 1,
            wait_for_client_justified_checkpoint: false,
            use_checkpoint_sync: false,
            connect_client_to_lean_spec_mesh: true,
            client_role: ClientUnderTestRole::Validator,
            source_helper_validator_indices: Some(
                LEAN_SPEC_SOURCE_VALIDATORS_EXCLUDING_V0.to_string(),
            ),
            split_helper_validators_across_mesh: true,
            helper_peer_count: 2,
            helper_fork_digest_profile: HelperGossipForkDigestProfile::SelectedDevnet,
        };

        assert_eq!(
            helper_mesh_validator_assignments(&test_data, 2),
            vec!["1,2".to_string(), "3".to_string()]
        );
        assert_eq!(helper_genesis_validator_count(&test_data), 4);
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
