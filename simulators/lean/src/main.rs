#![warn(clippy::unwrap_used)]

mod scenarios;

use std::{collections::HashMap, env, fmt, fs, time::Duration};

use crate::scenarios::rpc_compat::run_rpc_compat_lean_test_suite;
use crate::scenarios::sync::run_sync_lean_test_suite;
use alloy_primitives::B256;
use hivesim::types::ClientDefinition;
use hivesim::{run_suite, Client, Simulation, Suite, TestSpec};
use reqwest::Client as HttpClient;
use serde::{de::DeserializeOwned, Deserialize};
use tokio::time::sleep;

const HIVE_CHECK_LIVE_PORT: &str = "HIVE_CHECK_LIVE_PORT";
const HIVE_LEAN_DEVNET_LABEL: &str = "HIVE_LEAN_DEVNET_LABEL";
const LEAN_HTTP_PORT: u16 = 5052;
const LEAN_DEVNET_CONFIG_PATH: &str = "/app/hive/lean-devnets.txt";
const LEAN_ROLE: &str = "lean";
const HEALTHY_STATUS: &str = "healthy";
const LEAN_RPC_SERVICE: &str = "lean-rpc-api";

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
struct HealthResponse {
    status: String,
    service: String,
}

#[derive(Debug, Deserialize)]
struct CheckpointResponse {
    slot: u64,
    root: B256,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let simulation = Simulation::new();
    let devnet = resolve_selected_lean_devnet(&simulation).await;
    env::set_var(HIVE_LEAN_DEVNET_LABEL, devnet.as_str());

    let mut rpc_compat = Suite {
        name: "rpc-compat".to_string(),
        description: format!(
            "Runs Lean RPC compatibility tests against the selected lean clients using the {} profile.",
            devnet
        ),
        tests: vec![],
    };

    rpc_compat.add(TestSpec {
        name: "rpc-compat: client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: true,
        run: run_rpc_compat_lean_test_suite,
        client: None,
    });

    let mut sync = Suite {
        name: "sync".to_string(),
        description: format!(
            "Runs Lean sync tests against the selected lean clients using the {} profile.",
            devnet
        ),
        tests: vec![],
    };

    sync.add(TestSpec {
        name: "sync: client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: true,
        run: run_sync_lean_test_suite,
        client: None,
    });

    run_suite(simulation, vec![rpc_compat, sync]).await;
}

fn lean_clients(clients: Vec<ClientDefinition>) -> Vec<ClientDefinition> {
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

async fn resolve_selected_lean_devnet(simulation: &Simulation) -> LeanDevnet {
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
            None => {
                resolved_devnet = Some(devnet);
            }
        }
    }

    resolved_devnet.unwrap_or(config.default_devnet)
}

fn lean_environment() -> HashMap<String, String> {
    let devnet_label = selected_lean_devnet().to_string();
    HashMap::from([
        (HIVE_CHECK_LIVE_PORT.to_string(), LEAN_HTTP_PORT.to_string()),
        (HIVE_LEAN_DEVNET_LABEL.to_string(), devnet_label),
    ])
}

fn lean_api_url(client: &Client, path: &str) -> String {
    format!("http://{}:{}{}", client.ip, LEAN_HTTP_PORT, path)
}

async fn get_json_with_retry<T: DeserializeOwned>(http: &HttpClient, url: &str) -> T {
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
