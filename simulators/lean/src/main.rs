#![warn(clippy::unwrap_used)]

mod scenarios;

use std::{collections::HashMap, time::Duration};

use crate::scenarios::rpc_compat::run_rpc_compat_lean_test_suite;
use hivesim::types::ClientDefinition;
use hivesim::{run_suite, Client, Simulation, Suite, TestSpec};
use reqwest::Client as HttpClient;
use serde::{de::DeserializeOwned, Deserialize};
use tokio::time::sleep;

const HIVE_CHECK_LIVE_PORT: &str = "HIVE_CHECK_LIVE_PORT";
const HIVE_LEAN_DEVNET_LABEL: &str = "HIVE_LEAN_DEVNET_LABEL";
const LEAN_HTTP_PORT: u16 = 5052;
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
    fn label(self) -> &'static str {
        match self {
            Self::Devnet3 => "devnet3",
            Self::Devnet4 => "devnet4",
        }
    }
}

#[derive(Debug, Deserialize)]
struct HealthResponse {
    status: String,
    service: String,
}

#[derive(Debug, Deserialize)]
struct CheckpointResponse {
    slot: u64,
    root: String,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let devnet_label = selected_lean_devnet_label();

    let mut rpc_compat = Suite {
        name: "rpc-compat".to_string(),
        description: format!(
            "Runs Lean RPC compatibility tests against the selected lean clients using the {} profile.",
            devnet_label
        ),
        tests: vec![],
    };

    rpc_compat.add(TestSpec {
        name: "client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: false,
        run: run_rpc_compat_lean_test_suite,
        client: None,
    });

    run_suite(Simulation::new(), vec![rpc_compat]).await;
}

fn lean_clients(clients: Vec<ClientDefinition>) -> Vec<ClientDefinition> {
    clients
        .into_iter()
        .filter(|client| client.meta.roles.iter().any(|role| role == LEAN_ROLE))
        .collect()
}

// This is statically done in code right now for the sake of getting a working version, later it will be changed to allow for a flag to specify
// which devnet is being tested
pub(crate) fn selected_lean_devnet() -> LeanDevnet {
    match include_str!("../devnet.txt").trim() {
        "devnet3" => LeanDevnet::Devnet3,
        "devnet4" => LeanDevnet::Devnet4,
        other => {
            panic!("Unsupported Lean devnet selection `{other}` in simulators/lean/devnet.txt")
        }
    }
}

pub(crate) fn selected_lean_devnet_label() -> &'static str {
    selected_lean_devnet().label()
}

fn lean_environment() -> HashMap<String, String> {
    let devnet_label = selected_lean_devnet_label();
    HashMap::from([
        (HIVE_CHECK_LIVE_PORT.to_string(), LEAN_HTTP_PORT.to_string()),
        (HIVE_LEAN_DEVNET_LABEL.to_string(), devnet_label.to_string()),
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
