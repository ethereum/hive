#![warn(clippy::unwrap_used)]

use std::{collections::HashMap, time::Duration};

use hivesim::dyn_async;
use hivesim::types::ClientDefinition;
use hivesim::{run_suite, Client, NClientTestSpec, Simulation, Suite, Test, TestSpec};
use reqwest::Client as HttpClient;
use serde::{de::DeserializeOwned, Deserialize};
use tokio::time::sleep;

const HIVE_CHECK_LIVE_PORT: &str = "HIVE_CHECK_LIVE_PORT";
const LEAN_HTTP_PORT: u16 = 5052;
const LEAN_ROLE: &str = "lean";
const HEALTHY_STATUS: &str = "healthy";
const LEAN_RPC_SERVICE: &str = "lean-rpc-api";

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

    let mut api_smoke = Suite {
        name: "api-smoke".to_string(),
        description: "Launches lean clients and checks the shared onboarding API endpoints."
            .to_string(),
        tests: vec![],
    };

    api_smoke.add(TestSpec {
        name: "client launch".to_string(),
        description:
            "Starts each selected lean client and verifies its health and justified checkpoint endpoints."
                .to_string(),
        always_run: false,
        run: run_api_smoke_suite,
        client: None,
    });

    run_suite(Simulation::new(), vec![api_smoke]).await;
}

fn lean_clients(clients: Vec<ClientDefinition>) -> Vec<ClientDefinition> {
    clients
        .into_iter()
        .filter(|client| client.meta.roles.iter().any(|role| role == LEAN_ROLE))
        .collect()
}

fn lean_environment() -> HashMap<String, String> {
    HashMap::from([(HIVE_CHECK_LIVE_PORT.to_string(), LEAN_HTTP_PORT.to_string())])
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

dyn_async! {
    async fn run_api_smoke_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        for client in clients {
            test.run(NClientTestSpec {
                name: format!("basic api smoke {}", client.name),
                description: "Checks the health and justified checkpoint endpoints for a single lean client.".to_string(),
                always_run: false,
                run: test_basic_api_smoke,
                environments: Some(vec![Some(lean_environment())]),
                test_data: (),
                clients: vec![client],
            }).await;
        }
    }
}

dyn_async! {
    async fn test_basic_api_smoke<'a>(clients: Vec<Client>, _: ()) {
        let client = clients
            .into_iter()
            .next()
            .expect("NClientTestSpec should start exactly one client");

        let http = HttpClient::builder()
            .timeout(Duration::from_secs(5))
            .build()
            .expect("Unable to build HTTP client");

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

        let checkpoint: CheckpointResponse = get_json_with_retry(
            &http,
            &lean_api_url(&client, "/lean/v0/checkpoints/justified"),
        )
        .await;
        assert!(
            checkpoint.root.starts_with("0x"),
            "justified checkpoint root should be 0x-prefixed, got {}",
            checkpoint.root
        );
        assert_eq!(
            checkpoint.root.len(),
            66,
            "justified checkpoint root should be 32 bytes of hex plus 0x prefix"
        );
        assert_eq!(
            checkpoint.slot, 0,
            "a freshly started lean node should report the genesis justified checkpoint"
        );
    }
}
