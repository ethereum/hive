#![warn(clippy::unwrap_used)]

use std::{collections::HashMap, future::Future, time::Duration};

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
const REQUEST_TIMEOUT: Duration = Duration::from_secs(5);
const REQUEST_RETRIES: usize = 30;
const REQUEST_RETRY_DELAY: Duration = Duration::from_secs(1);
const STARTUP_STABILITY_POLLS: usize = 6;
const STARTUP_STABILITY_DELAY: Duration = Duration::from_secs(2);
const ROOT_HEX_LENGTH: usize = 66;
const SSZ_ACCEPT_HEADER: &str = "application/octet-stream";

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

#[derive(Debug, Deserialize)]
struct ForkChoiceNode {
    slot: u64,
    root: String,
}

#[derive(Debug, Deserialize)]
struct ForkChoiceResponse {
    head: ForkChoiceHead,
    justified: ForkChoiceNode,
    finalized: ForkChoiceNode,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum ForkChoiceHead {
    Root(String),
    Node(ForkChoiceNode),
}

#[derive(Debug)]
struct BinaryResponse {
    content_type: String,
    body: Vec<u8>,
}

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let simulation = Simulation::new();

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

    let mut api_compat = Suite {
        name: "api-compat".to_string(),
        description: "Checks shared lean API compatibility across all selected lean clients."
            .to_string(),
        tests: vec![],
    };

    api_compat.add(TestSpec {
        name: "finalized-state-ssz".to_string(),
        description:
            "Verifies that each selected lean client serves a non-empty finalized state SSZ payload."
                .to_string(),
        always_run: false,
        run: run_finalized_state_ssz_suite,
        client: None,
    });

    api_compat.add(TestSpec {
        name: "fork-choice-shape".to_string(),
        description:
            "Verifies that each selected lean client exposes the shared fork choice JSON shape."
                .to_string(),
        always_run: false,
        run: run_fork_choice_shape_suite,
        client: None,
    });

    api_compat.add(TestSpec {
        name: "startup-stability".to_string(),
        description:
            "Polls the fork choice and justified checkpoint endpoints after startup to ensure they remain stable."
                .to_string(),
        always_run: false,
        run: run_startup_stability_suite,
        client: None,
    });

    run_suite(simulation, vec![api_smoke, api_compat]).await;
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

fn build_http_client() -> HttpClient {
    HttpClient::builder()
        .timeout(REQUEST_TIMEOUT)
        .build()
        .expect("Unable to build HTTP client")
}

async fn get_json_with_retry<T: DeserializeOwned>(http: &HttpClient, url: &str) -> T {
    let mut last_error = String::new();

    for attempt in 1..=REQUEST_RETRIES {
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

        if attempt < REQUEST_RETRIES {
            sleep(REQUEST_RETRY_DELAY).await;
        }
    }

    panic!("Request to {url} did not succeed after retries: {last_error}");
}

async fn get_binary_with_retry(
    http: &HttpClient,
    url: &str,
    accept: Option<&str>,
) -> BinaryResponse {
    let mut last_error = String::new();

    for attempt in 1..=REQUEST_RETRIES {
        let mut request = http.get(url);
        if let Some(accept) = accept {
            request = request.header(reqwest::header::ACCEPT, accept);
        }

        match request.send().await {
            Ok(response) => {
                let status = response.status();
                if !status.is_success() {
                    last_error = format!("received HTTP {status} from {url}");
                } else {
                    let content_type = response
                        .headers()
                        .get(reqwest::header::CONTENT_TYPE)
                        .and_then(|value| value.to_str().ok())
                        .unwrap_or_default()
                        .to_string();
                    let body = response.bytes().await.unwrap_or_else(|err| {
                        panic!("Unable to read binary response body from {url}: {err}")
                    });

                    return BinaryResponse {
                        content_type,
                        body: body.to_vec(),
                    };
                }
            }
            Err(err) => {
                last_error = err.to_string();
            }
        }

        if attempt < REQUEST_RETRIES {
            sleep(REQUEST_RETRY_DELAY).await;
        }
    }

    panic!("Request to {url} did not succeed after retries: {last_error}");
}

fn assert_valid_root(root: &str, context: &str) {
    assert!(
        root.starts_with("0x"),
        "{context} root should be 0x-prefixed, got {root}"
    );
    assert_eq!(
        root.len(),
        ROOT_HEX_LENGTH,
        "{context} root should be 32 bytes of hex plus the 0x prefix"
    );
    assert!(
        root[2..]
            .chars()
            .all(|character| character.is_ascii_hexdigit()),
        "{context} root should contain only hexadecimal characters, got {root}"
    );
}

fn assert_checkpoint_shape(checkpoint: &CheckpointResponse, context: &str) {
    assert_valid_root(&checkpoint.root, context);
}

fn assert_fork_choice_shape(fork_choice: &ForkChoiceResponse, context: &str) {
    match &fork_choice.head {
        ForkChoiceHead::Root(root) => assert_valid_root(root, &format!("{context} head")),
        ForkChoiceHead::Node(node) => assert_valid_root(&node.root, &format!("{context} head")),
    }
    assert_valid_root(&fork_choice.justified.root, &format!("{context} justified"));
    assert_valid_root(&fork_choice.finalized.root, &format!("{context} finalized"));

    if let ForkChoiceHead::Node(node) = &fork_choice.head {
        let _ = node.slot;
    }
    let _ = fork_choice.justified.slot;
    let _ = fork_choice.finalized.slot;
}

async fn wait_for_healthy(http: &HttpClient, client: &Client) {
    let health: HealthResponse =
        get_json_with_retry(http, &lean_api_url(client, "/lean/v0/health")).await;
    assert_eq!(
        health.status, HEALTHY_STATUS,
        "{} reported an unexpected health status",
        client.kind
    );
}

async fn poll_with_delay<F, Fut>(polls: usize, delay: Duration, mut action: F)
where
    F: FnMut(usize) -> Fut,
    Fut: Future<Output = ()>,
{
    for poll_number in 1..=polls {
        action(poll_number).await;
        if poll_number < polls {
            sleep(delay).await;
        }
    }
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

        let http = build_http_client();

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

dyn_async! {
    async fn run_finalized_state_ssz_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        let http = build_http_client();

        for client_definition in clients {
            let client_name = client_definition.name.clone();
            let client = test
                .start_client(client_definition.name, Some(lean_environment()))
                .await;

            wait_for_healthy(&http, &client).await;

            let finalized_state = get_binary_with_retry(
                &http,
                &lean_api_url(&client, "/lean/v0/states/finalized"),
                Some(SSZ_ACCEPT_HEADER),
            )
            .await;

            let content_type = finalized_state.content_type.to_ascii_lowercase();
            assert!(
                content_type.contains("application/octet-stream") || content_type.contains("ssz"),
                "{} returned an unexpected content type for /lean/v0/states/finalized: {}",
                client_name,
                finalized_state.content_type
            );
            assert!(
                !finalized_state.body.is_empty(),
                "{} returned an empty finalized state body",
                client_name
            );
        }
    }
}

dyn_async! {
    async fn run_fork_choice_shape_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        let http = build_http_client();

        for client_definition in clients {
            let client_name = client_definition.name.clone();
            let client = test
                .start_client(client_definition.name, Some(lean_environment()))
                .await;

            wait_for_healthy(&http, &client).await;

            let fork_choice: ForkChoiceResponse = get_json_with_retry(
                &http,
                &lean_api_url(&client, "/lean/v0/fork_choice"),
            )
            .await;
            assert_fork_choice_shape(&fork_choice, &format!("{client_name} fork choice"));
        }
    }
}

dyn_async! {
    async fn run_startup_stability_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

        let http = build_http_client();

        for client_definition in clients {
            let client_name = client_definition.name.clone();
            let client = test
                .start_client(client_definition.name, Some(lean_environment()))
                .await;

            wait_for_healthy(&http, &client).await;

            let fork_choice_url = lean_api_url(&client, "/lean/v0/fork_choice");
            let checkpoint_url = lean_api_url(&client, "/lean/v0/checkpoints/justified");

            poll_with_delay(STARTUP_STABILITY_POLLS, STARTUP_STABILITY_DELAY, |poll_number| {
                let http = http.clone();
                let client_name = client_name.clone();
                let fork_choice_url = fork_choice_url.clone();
                let checkpoint_url = checkpoint_url.clone();

                async move {
                    let fork_choice: ForkChoiceResponse =
                        get_json_with_retry(&http, &fork_choice_url).await;
                    assert_fork_choice_shape(
                        &fork_choice,
                        &format!("{client_name} fork choice poll {poll_number}"),
                    );

                    let checkpoint: CheckpointResponse =
                        get_json_with_retry(&http, &checkpoint_url).await;
                    assert_checkpoint_shape(
                        &checkpoint,
                        &format!("{client_name} justified checkpoint poll {poll_number}"),
                    );
                }
            })
            .await;
        }
    }
}
