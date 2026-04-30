//! General Lean scenario utilities shared across suites.

use std::collections::HashMap;
use std::env;
use std::fmt;
use std::fs;
use std::future::Future;
use std::net::{IpAddr, Ipv4Addr, UdpSocket};
use std::path::{Path, PathBuf};
use std::pin::Pin;
use std::process::Command;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use alloy_primitives::B256;
use hivesim::types::ClientDefinition;
use hivesim::{types::TestResult, Client, Simulation, Test};
use reqwest::{header::ACCEPT, Client as HttpClient, Url};
use serde::{de::DeserializeOwned, Deserialize};
use tokio::time::{sleep, timeout};

const HIVE_CHECK_LIVE_PORT: &str = "HIVE_CHECK_LIVE_PORT";
const HIVE_LEAN_DEVNET_LABEL: &str = "HIVE_LEAN_DEVNET_LABEL";
const HIVE_LEAN_FORK_DIGEST: &str = "HIVE_LEAN_FORK_DIGEST";
const LEAN_HTTP_PORT: u16 = 5052;
const LEAN_DEVNET_CONFIG_PATH: &str = "/app/hive/lean-devnets.txt";
const LEAN_ROLE: &str = "lean";
pub(crate) const LEAN_HELPER_ADVERTISE_IP_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_ADVERTISE_IP";
pub(crate) const LEAN_HELPER_GOSSIP_FORK_DIGEST_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_GOSSIP_FORK_DIGEST";
pub(crate) const LEAN_HELPER_P2P_PORT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_P2P_PORT";
pub(crate) const LEAN_HELPER_API_PORT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_HELPER_API_PORT";
pub(crate) const LEAN_HELPER_METADATA_PORT_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_METADATA_PORT";
pub(crate) const LEAN_HELPER_IDENTITY_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str =
    "HIVE_LEAN_HELPER_IDENTITY_PRIVATE_KEY";
const LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_CLIENT_RUNTIME_ROLE";
const CLIENT_RUNTIME_ASSET_PREPARER: &str = "/app/hive/prepare_lean_client_assets.py";
const LEAN_GENESIS_DELAY_SECS: u64 = 30;
pub(crate) const DEVNET4_HELPER_GOSSIP_FORK_DIGEST: &str = "12345678";
pub(crate) const HEALTHY_STATUS: &str = "healthy";
pub(crate) const LEAN_RPC_SERVICE: &str = "lean-rpc-api";

#[allow(dead_code)]
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub(crate) enum LeanDevnet {
    Devnet3,
    Devnet4,
}

impl LeanDevnet {
    const DEFAULT: Self = Self::Devnet3;

    fn as_str(self) -> &'static str {
        match self {
            Self::Devnet3 => "devnet3",
            Self::Devnet4 => "devnet4",
        }
    }
}

impl fmt::Display for LeanDevnet {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str(self.as_str())
    }
}

impl TryFrom<&str> for LeanDevnet {
    type Error = String;

    fn try_from(label: &str) -> Result<Self, Self::Error> {
        match label.trim() {
            "devnet3" => Ok(Self::Devnet3),
            "devnet4" => Ok(Self::Devnet4),
            other => Err(format!("unsupported lean devnet label {other:?}")),
        }
    }
}

struct LeanDevnetConfig {
    default_devnet: LeanDevnet,
    client_support: HashMap<String, Vec<LeanDevnet>>,
}

#[derive(Debug, Deserialize)]
pub(crate) struct HealthResponse {
    pub(crate) status: String,
    pub(crate) service: String,
}

#[derive(Clone, Debug, Deserialize)]
pub(crate) struct CheckpointResponse {
    pub(crate) slot: u64,
    pub(crate) root: B256,
}

#[derive(Deserialize)]
pub(crate) struct LeanBootnodeMetadata {
    pub(crate) enr: String,
    #[serde(default)]
    pub(crate) qlean_enr: Option<String>,
    pub(crate) multiaddr: String,
}

#[derive(Clone, Copy)]
pub(crate) enum ClientUnderTestRole {
    Validator,
    Observer,
}

impl ClientUnderTestRole {
    pub(crate) fn apply_to_environment(self, environment: &mut HashMap<String, String>) {
        if matches!(self, Self::Observer) {
            environment.insert(
                LEAN_CLIENT_RUNTIME_ROLE_ENVIRONMENT_VARIABLE.to_string(),
                "observer".to_string(),
            );
        }
    }
}

#[derive(Clone, Debug, Deserialize)]
pub(crate) struct ForkChoiceSnapshot {
    pub justified: CheckpointResponse,
    pub finalized: CheckpointResponse,
}

#[derive(Clone, Debug, Deserialize)]
pub(crate) struct ForkChoiceNodeResponse {
    pub root: B256,
    pub slot: u64,
    pub parent_root: B256,
    pub proposer_index: u64,
    pub weight: u64,
}

#[derive(Clone, Debug, Deserialize)]
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

pub(crate) fn lean_clients(clients: Vec<ClientDefinition>) -> Vec<ClientDefinition> {
    clients
        .into_iter()
        .filter(|client| client.meta.roles.iter().any(|role| role == LEAN_ROLE))
        .collect()
}

pub(crate) fn selected_lean_devnet() -> LeanDevnet {
    let label =
        env::var(HIVE_LEAN_DEVNET_LABEL).unwrap_or_else(|_| LeanDevnet::DEFAULT.to_string());
    LeanDevnet::try_from(label.as_str()).unwrap_or_else(|err| {
        panic!(
            "Unsupported Lean devnet selection in environment variable {HIVE_LEAN_DEVNET_LABEL}: {err}"
        )
    })
}

pub(crate) fn set_selected_lean_devnet(devnet: LeanDevnet) {
    env::set_var(HIVE_LEAN_DEVNET_LABEL, devnet.as_str());
}

pub(crate) async fn resolve_selected_lean_devnet(simulation: &Simulation) -> LeanDevnet {
    let config = load_lean_devnet_config();
    let clients = lean_clients(simulation.client_types().await);
    if clients.is_empty() {
        return config.default_devnet;
    }

    let mut resolved_devnet = None;
    let mut selected_labels = Vec::new();

    for client in clients {
        let (base_name, explicit_devnet) = split_client_devnet_name(&client.name);
        let devnet = explicit_devnet.unwrap_or(config.default_devnet);
        let supported_devnets = config.client_support.get(base_name).unwrap_or_else(|| {
            panic!(
                "Lean client {} is missing from {}",
                base_name, LEAN_DEVNET_CONFIG_PATH
            )
        });

        if !supported_devnets.contains(&devnet) {
            let supported = supported_devnets
                .iter()
                .map(ToString::to_string)
                .collect::<Vec<_>>()
                .join(", ");
            panic!(
                "Lean client {} does not support {} according to {} (supported: {})",
                client.name, devnet, LEAN_DEVNET_CONFIG_PATH, supported,
            );
        }

        selected_labels.push(format!("{}={devnet}", client.name));
        match resolved_devnet {
            Some(previous) if previous != devnet => {
                panic!(
                    "Mixed lean devnets selected in one run: {}",
                    selected_labels.join(", ")
                );
            }
            Some(_) => {}
            None => resolved_devnet = Some(devnet),
        }
    }

    resolved_devnet.unwrap_or(config.default_devnet)
}

pub(crate) fn lean_environment() -> HashMap<String, String> {
    let selected_devnet = selected_lean_devnet();
    let devnet_label = selected_devnet.to_string();
    let mut environment = HashMap::from([
        (HIVE_CHECK_LIVE_PORT.to_string(), LEAN_HTTP_PORT.to_string()),
        (HIVE_LEAN_DEVNET_LABEL.to_string(), devnet_label),
    ]);

    if selected_devnet == LeanDevnet::Devnet4 {
        environment.insert(
            HIVE_LEAN_FORK_DIGEST.to_string(),
            DEVNET4_HELPER_GOSSIP_FORK_DIGEST.to_string(),
        );
    }

    environment
}

pub(crate) fn lean_api_url(client: &Client, path: &str) -> String {
    format!("http://{}:{}{}", client.ip, LEAN_HTTP_PORT, path)
}

pub(crate) fn lean_bootnodes_for_client(
    client_kind: &str,
    bootnodes: &[LeanBootnodeMetadata],
) -> String {
    if bootnodes.is_empty() {
        return "none".to_string();
    }

    bootnodes
        .iter()
        .map(|bootnode| {
            if client_uses_enr_bootnodes(client_kind) {
                bootnode_enr_for_client(
                    client_kind,
                    Some(bootnode.enr.as_str()),
                    bootnode.qlean_enr.as_deref(),
                )
                .unwrap_or(bootnode.enr.as_str())
            } else {
                bootnode.multiaddr.as_str()
            }
        })
        .collect::<Vec<_>>()
        .join(",")
}

pub(crate) fn bootnode_enr_for_client<'a>(
    client_kind: &str,
    enr: Option<&'a str>,
    qlean_enr: Option<&'a str>,
) -> Option<&'a str> {
    if client_kind.starts_with("qlean") {
        return qlean_enr.or(enr);
    }

    enr
}

pub(crate) fn client_uses_enr_bootnodes(client_kind: &str) -> bool {
    client_kind.starts_with("ethlambda")
        || client_kind.starts_with("lantern")
        || client_kind.starts_with("qlean")
        || client_kind.starts_with("zeam")
}

pub(crate) fn bootnode_metadata_for_client(
    private_key: &str,
    ip: IpAddr,
    p2p_port: u16,
) -> LeanBootnodeMetadata {
    let private_key_bytes = parse_secp256k1_private_key(private_key);
    let ip = lean_bootnode_ip4(ip);
    let peer_id = peer_id_for_private_key(private_key_bytes);

    LeanBootnodeMetadata {
        enr: build_lean_bootnode_enr(private_key_bytes, ip, p2p_port, true),
        qlean_enr: Some(build_lean_bootnode_enr(
            private_key_bytes,
            ip,
            p2p_port,
            false,
        )),
        multiaddr: format!("/ip4/{ip}/udp/{p2p_port}/quic-v1/p2p/{peer_id}"),
    }
}

fn lean_bootnode_ip4(ip: IpAddr) -> Ipv4Addr {
    match ip {
        IpAddr::V4(ip) => ip,
        IpAddr::V6(ip) => {
            panic!("Lean bootnode metadata currently requires an IPv4 address, got {ip}")
        }
    }
}

fn parse_secp256k1_private_key(private_key: &str) -> [u8; 32] {
    let private_key = private_key
        .trim()
        .strip_prefix("0x")
        .unwrap_or_else(|| private_key.trim());
    if private_key.len() != 64 {
        panic!(
            "Lean secp256k1 private keys must be 32 bytes encoded as 64 hex characters, got {}",
            private_key.len()
        );
    }

    let mut bytes = [0_u8; 32];
    for (index, byte) in bytes.iter_mut().enumerate() {
        let offset = index * 2;
        let high = hex_digit(private_key.as_bytes()[offset]).unwrap_or_else(|| {
            panic!(
                "Invalid hex character {:?} in lean secp256k1 private key",
                private_key.as_bytes()[offset] as char
            )
        });
        let low = hex_digit(private_key.as_bytes()[offset + 1]).unwrap_or_else(|| {
            panic!(
                "Invalid hex character {:?} in lean secp256k1 private key",
                private_key.as_bytes()[offset + 1] as char
            )
        });
        *byte = (high << 4) | low;
    }

    bytes
}

fn hex_digit(byte: u8) -> Option<u8> {
    match byte {
        b'0'..=b'9' => Some(byte - b'0'),
        b'a'..=b'f' => Some(byte - b'a' + 10),
        b'A'..=b'F' => Some(byte - b'A' + 10),
        _ => None,
    }
}

fn peer_id_for_private_key(mut private_key: [u8; 32]) -> String {
    let secret_key = libp2p_identity::secp256k1::SecretKey::try_from_bytes(&mut private_key)
        .unwrap_or_else(|err| panic!("Unable to parse lean secp256k1 private key: {err}"));
    let secp_keypair = libp2p_identity::secp256k1::Keypair::from(secret_key);
    let keypair = libp2p_identity::Keypair::from(secp_keypair);
    keypair.public().to_peer_id().to_string()
}

fn build_lean_bootnode_enr(
    private_key: [u8; 32],
    ip: Ipv4Addr,
    p2p_port: u16,
    include_udp: bool,
) -> String {
    let signing_key = enr::k256::ecdsa::SigningKey::from_slice(&private_key)
        .unwrap_or_else(|err| panic!("Unable to parse lean ENR signing key: {err}"));
    let mut builder = enr::Enr::<enr::k256::ecdsa::SigningKey>::builder();
    builder.ip4(ip);
    if include_udp {
        builder.udp4(p2p_port);
    }
    builder.add_value("quic", &p2p_port);

    builder
        .build(&signing_key)
        .unwrap_or_else(|err| panic!("Unable to build lean bootnode ENR: {err}"))
        .to_base64()
}

pub(crate) async fn get_json_with_retry<T: DeserializeOwned>(http: &HttpClient, url: &str) -> T {
    let mut last_error = String::new();

    for attempt in 1..=30 {
        match http.get(url).send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    return response.json::<T>().await.unwrap_or_else(|err| {
                        panic!("Unable to decode response from {url}: {err}")
                    });
                }
            }
            Err(err) => {
                last_error = err.to_string();
            }
        }

        if attempt < 30 {
            sleep(Duration::from_secs(1)).await;
        }
    }

    panic!("Request to {url} did not succeed after retries: {last_error}");
}

fn load_lean_devnet_config() -> LeanDevnetConfig {
    let contents = fs::read_to_string(LEAN_DEVNET_CONFIG_PATH).unwrap_or_else(|err| {
        panic!("Unable to read lean devnet config {LEAN_DEVNET_CONFIG_PATH}: {err}")
    });

    let mut default_devnet = None;
    let mut client_support = HashMap::new();

    for (index, raw_line) in contents.lines().enumerate() {
        let line_number = index + 1;
        let line = raw_line
            .split('#')
            .next()
            .expect("split always yields at least one segment")
            .trim();
        if line.is_empty() {
            continue;
        }

        let (key, value) = line.split_once('=').unwrap_or_else(|| {
            panic!(
                "Invalid lean devnet config line {} in {}: expected key=value, got {:?}",
                line_number, LEAN_DEVNET_CONFIG_PATH, raw_line
            )
        });
        let key = key.trim();
        let value = value.trim();

        if key == "default" {
            default_devnet = Some(LeanDevnet::try_from(value).unwrap_or_else(|err| {
                panic!(
                    "Invalid default lean devnet in {} line {}: {}",
                    LEAN_DEVNET_CONFIG_PATH, line_number, err
                )
            }));
            continue;
        }

        let supported_devnets = value
            .split(',')
            .map(|label| {
                LeanDevnet::try_from(label).unwrap_or_else(|err| {
                    panic!(
                        "Invalid lean devnet in {} line {} for client {}: {}",
                        LEAN_DEVNET_CONFIG_PATH, line_number, key, err
                    )
                })
            })
            .collect::<Vec<_>>();

        if supported_devnets.is_empty() {
            panic!(
                "Lean devnet config {} line {} defines client {} without any supported devnets",
                LEAN_DEVNET_CONFIG_PATH, line_number, key
            );
        }

        client_support.insert(key.to_string(), supported_devnets);
    }

    LeanDevnetConfig {
        default_devnet: default_devnet.unwrap_or_else(|| {
            panic!(
                "Lean devnet config {} must define a default=<devnet> entry",
                LEAN_DEVNET_CONFIG_PATH
            )
        }),
        client_support,
    }
}

fn split_client_devnet_name(client_name: &str) -> (&str, Option<LeanDevnet>) {
    let Some((base_name, suffix)) = client_name.rsplit_once('_') else {
        return (client_name, None);
    };
    match LeanDevnet::try_from(suffix) {
        Ok(devnet) => (base_name, Some(devnet)),
        Err(_) => (client_name, None),
    }
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
            join_handle.await.ok();
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
    current_unix_time() + LEAN_GENESIS_DELAY_SECS
}

pub(crate) fn current_unix_time() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("System time is before UNIX_EPOCH")
        .as_secs()
}

pub(crate) fn lean_client_kind(client_type: &str) -> Result<&'static str, String> {
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
        fs::remove_dir_all(&local_root).ok();
        return Err(format!(
            "{CLIENT_RUNTIME_ASSET_PREPARER} failed for {client_type} with status {}.\nstdout:\n{}\nstderr:\n{}",
            output.status, stdout, stderr
        ));
    }

    let mut files = HashMap::new();
    collect_client_runtime_files(&local_root, &local_root, &container_root, &mut files)?;
    fs::remove_dir_all(&local_root).ok();
    Ok(files)
}

pub(crate) fn panic_payload_to_string(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(error) = payload.downcast_ref::<&'static str>() {
        error.to_string()
    } else if let Some(error) = payload.downcast_ref::<String>() {
        error.clone()
    } else {
        format!("unknown panic payload: {payload:?}")
    }
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

pub(crate) fn simulator_container_ip() -> IpAddr {
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
