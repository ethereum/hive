use crate::utils::util::{
    bootnode_metadata_for_client, current_unix_time, default_genesis_time, http_client,
    lean_api_url, lean_bootnodes_for_client, lean_client_kind, lean_clients, lean_environment,
    panic_payload_to_string, prepare_client_runtime_files, run_data_test_with_timeout,
    ForkChoiceResponse, LeanBootnodeMetadata, TimedDataTestSpec,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, Test};
use std::collections::{HashMap, HashSet};
use std::time::Duration;
use tokio::time::{sleep, timeout};

const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE: &str =
    "HIVE_ATTESTATION_COMMITTEE_COUNT";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE: &str = "HIVE_VALIDATOR_INDEX";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";

const CLIENT_INTEROP_NODE_COUNT: usize = 3;
const CLIENT_INTEROP_P2P_PORT: u16 = 9000;
const FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS: u64 = 3 * 60;
const CLIENT_STARTUP_ATTEMPTS: u64 = 3;
const CLIENT_STARTUP_TIMEOUT_SECS: u64 = 120;
const OUTER_TEST_TIMEOUT_GRACE_SECS: u64 = 750;
const FINALIZATION_POLL_INTERVAL_SECS: u64 = 1;

#[derive(Clone)]
struct ClientInteropNode {
    client: ClientDefinition,
    client_kind: String,
    node_id: String,
    validator_index: usize,
    private_key: String,
    is_aggregator: bool,
}

#[derive(Clone)]
struct ClientInteropTestData {
    run_label: String,
    nodes: Vec<ClientInteropNode>,
    genesis_time: u64,
}

struct ClientInteropTopologySpec {
    left_name: String,
    right_name: String,
    topology: Vec<ClientDefinition>,
}

struct RunningInteropClient {
    node_id: String,
    client_kind: String,
    client: Client,
}

struct FinalizationObservation {
    node_id: String,
    client_kind: String,
    slot: Option<u64>,
    error: Option<String>,
}

dyn_async! {
    pub async fn run_client_interop_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        assert!(!clients.is_empty(), "client-interop requires at least one selected lean client, got {}", clients.len());

        for topology_spec in interop_topology_matrix(&clients) {
            let mut nodes = build_interop_nodes(topology_spec.topology);
            assign_aggregator(&mut nodes);

            let genesis_time = default_genesis_time();
            let topology_label = topology_label(&nodes);
            let run_label = format!(
                "{} and {} / {}",
                topology_spec.left_name, topology_spec.right_name, topology_label
            );

            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name: format!("client-interop: {}", run_label),
                    description: format!(
                        "Starts {} with a shared genesis and checks that all three nodes finalize past genesis at the same slot.",
                        topology_label
                    ),
                    always_run: false,
                    client_name: run_label.clone(),
                    timeout_duration: outer_timeout_for_genesis(genesis_time),
                    test_data: ClientInteropTestData {
                        run_label,
                        nodes,
                        genesis_time,
                    },
                },
                test_client_interop_finalizes,
            )
            .await;
        }
    }
}

dyn_async! {
    async fn test_client_interop_finalizes<'a>(test: &'a mut Test, test_data: ClientInteropTestData) {
        let mut running_clients = Vec::with_capacity(CLIENT_INTEROP_NODE_COUNT);
        let mut bootnodes = Vec::with_capacity(CLIENT_INTEROP_NODE_COUNT);

        for node in &test_data.nodes {
            let client =
                start_interop_client(test, node, &bootnodes, test_data.genesis_time).await;
            let metadata =
                bootnode_metadata_for_client(&node.private_key, client.ip, CLIENT_INTEROP_P2P_PORT);
            bootnodes.push(metadata);
            running_clients.push(RunningInteropClient {
                node_id: node.node_id.clone(),
                client_kind: node.client_kind.clone(),
                client,
            });
        }

        wait_for_same_non_genesis_finalized_slot(
            &test_data.run_label,
            test_data.genesis_time,
            &running_clients,
        )
        .await;
    }
}

fn interop_topology_matrix(clients: &[ClientDefinition]) -> Vec<ClientInteropTopologySpec> {
    let mut topology_specs = Vec::new();
    let mut self_tested = HashSet::new();

    for left_index in 0..clients.len() {
        for right_index in left_index..clients.len() {
            let left = &clients[left_index];
            let right = &clients[right_index];

            if left.name == right.name && !self_tested.insert(left.name.clone()) {
                continue;
            }

            topology_specs.extend(interop_topologies(left, right).into_iter().map(|topology| {
                ClientInteropTopologySpec {
                    left_name: left.name.clone(),
                    right_name: right.name.clone(),
                    topology,
                }
            }));
        }
    }

    topology_specs
}

fn interop_topologies(
    left: &ClientDefinition,
    right: &ClientDefinition,
) -> Vec<Vec<ClientDefinition>> {
    if left.name == right.name {
        return vec![vec![left.clone(), left.clone(), left.clone()]];
    }

    vec![
        vec![left.clone(), left.clone(), right.clone()],
        vec![left.clone(), right.clone(), right.clone()],
    ]
}

fn build_interop_nodes(topology: Vec<ClientDefinition>) -> Vec<ClientInteropNode> {
    let mut client_kind_counts = HashMap::<String, usize>::new();
    topology
        .into_iter()
        .enumerate()
        .map(|(validator_index, client)| {
            let client_kind = lean_client_kind(&client.name)
                .unwrap_or_else(|err| panic!("Unable to build client-interop topology: {err}"))
                .to_string();
            let client_count = client_kind_counts.entry(client_kind.clone()).or_insert(0);
            let node_id = format!("{client_kind}_{client_count}");
            *client_count += 1;

            ClientInteropNode {
                client,
                client_kind,
                node_id,
                validator_index,
                private_key: private_key_for_node(validator_index),
                is_aggregator: false,
            }
        })
        .collect()
}

fn assign_aggregator(nodes: &mut [ClientInteropNode]) {
    let aggregator_index = nodes
        .iter()
        .position(|node| node.client_kind == "ream")
        .unwrap_or(0);

    for (index, node) in nodes.iter_mut().enumerate() {
        node.is_aggregator = index == aggregator_index;
    }
}

fn topology_label(nodes: &[ClientInteropNode]) -> String {
    nodes
        .iter()
        .map(|node| node.client.name.as_str())
        .collect::<Vec<_>>()
        .join(",")
}

fn private_key_for_node(index: usize) -> String {
    format!("{:064x}", index + 1)
}

async fn start_interop_client(
    test: &Test,
    node: &ClientInteropNode,
    bootnodes: &[LeanBootnodeMetadata],
    genesis_time: u64,
) -> Client {
    let environment = client_interop_environment(node, bootnodes, genesis_time);
    let files =
        prepare_client_runtime_files(&node.client.name, &environment).unwrap_or_else(|err| {
            panic!(
                "Unable to prepare runtime assets for client-interop node {} ({}): {err}",
                node.node_id, node.client.name
            )
        });
    let mut last_error = None;

    for attempt in 1..=CLIENT_STARTUP_ATTEMPTS {
        match start_interop_client_attempt(
            test,
            node.client.name.clone(),
            environment.clone(),
            files.clone(),
        )
        .await
        {
            Ok(client) => return client,
            Err(message) if attempt < CLIENT_STARTUP_ATTEMPTS => {
                eprintln!(
                    "Retrying client-interop startup for node {} ({}) after attempt {} failed: {}",
                    node.node_id, node.client.name, attempt, message
                );
                last_error = Some(message);
                sleep(Duration::from_secs(1)).await;
            }
            Err(message) => {
                panic!(
                    "Unable to start client-interop node {} ({}) after {} attempts: {}",
                    node.node_id, node.client.name, CLIENT_STARTUP_ATTEMPTS, message
                );
            }
        };
    }

    panic!(
        "Unable to start client-interop node {} ({}) after {} attempts{}",
        node.node_id,
        node.client.name,
        CLIENT_STARTUP_ATTEMPTS,
        last_error
            .map(|error| format!(": {error}"))
            .unwrap_or_default()
    );
}

async fn start_interop_client_attempt(
    test: &Test,
    client_name: String,
    environment: HashMap<String, String>,
    files: HashMap<String, Vec<u8>>,
) -> Result<Client, String> {
    let test = test.clone();
    let mut handle = tokio::spawn(async move {
        test.start_client_with_files(client_name, Some(environment), Some(files))
            .await
    });

    match timeout(
        Duration::from_secs(CLIENT_STARTUP_TIMEOUT_SECS),
        &mut handle,
    )
    .await
    {
        Ok(Ok(client)) => Ok(client),
        Ok(Err(err)) => {
            if err.is_panic() {
                Err(panic_payload_to_string(err.into_panic()))
            } else {
                Err(err.to_string())
            }
        }
        Err(_) => {
            handle.abort();
            handle.await.ok();
            Err(format!(
                "startup attempt exceeded {} seconds",
                CLIENT_STARTUP_TIMEOUT_SECS
            ))
        }
    }
}

fn client_interop_environment(
    node: &ClientInteropNode,
    bootnodes: &[LeanBootnodeMetadata],
    genesis_time: u64,
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    let validator_index = node.validator_index.to_string();

    environment.insert(
        BOOTNODES_ENVIRONMENT_VARIABLE.to_string(),
        lean_bootnodes_for_client(&node.client_kind, bootnodes),
    );
    environment.insert(
        CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(),
        node.private_key.clone(),
    );
    environment.insert(
        LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE.to_string(),
        "1".to_string(),
    );
    environment.insert(
        LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE.to_string(),
        genesis_time.to_string(),
    );
    environment.insert(
        LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE.to_string(),
        validator_index.clone(),
    );
    environment.insert(
        LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE.to_string(),
        validator_index,
    );
    environment.insert(
        NODE_ID_ENVIRONMENT_VARIABLE.to_string(),
        node.node_id.clone(),
    );

    if node.is_aggregator {
        environment.insert(
            IS_AGGREGATOR_ENVIRONMENT_VARIABLE.to_string(),
            "1".to_string(),
        );
    }

    environment
}

async fn wait_for_same_non_genesis_finalized_slot(
    run_label: &str,
    genesis_time: u64,
    clients: &[RunningInteropClient],
) {
    let deadline = genesis_time + FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS;
    let mut last_observations = Vec::new();

    while current_unix_time() <= deadline {
        let observations = load_finalization_observations(clients).await;
        if let Some(finalized_slot) = common_non_genesis_finalized_slot(&observations) {
            assert!(
                finalized_slot > 0,
                "client-interop finalized slot should be past genesis"
            );
            return;
        }

        last_observations = observations;
        sleep(Duration::from_secs(FINALIZATION_POLL_INTERVAL_SECS)).await;
    }

    panic!(
        "client-interop run {} did not observe all nodes finalized past genesis at the same slot within {} seconds after genesis_time {} (deadline {}). Last observations: {}",
        run_label,
        FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
        genesis_time,
        deadline,
        format_finalization_observations(&last_observations),
    );
}

async fn load_finalization_observations(
    clients: &[RunningInteropClient],
) -> Vec<FinalizationObservation> {
    let mut observations = Vec::with_capacity(clients.len());

    for running_client in clients {
        match try_load_finalized_slot(&running_client.client).await {
            Ok(slot) => observations.push(FinalizationObservation {
                node_id: running_client.node_id.clone(),
                client_kind: running_client.client_kind.clone(),
                slot: Some(slot),
                error: None,
            }),
            Err(error) => observations.push(FinalizationObservation {
                node_id: running_client.node_id.clone(),
                client_kind: running_client.client_kind.clone(),
                slot: None,
                error: Some(error),
            }),
        }
    }

    observations
}

async fn try_load_finalized_slot(client: &Client) -> Result<u64, String> {
    let url = lean_api_url(client, "/lean/v0/fork_choice");
    let response = http_client()
        .get(&url)
        .send()
        .await
        .map_err(|err| format!("error sending request for url ({url}): {err}"))?;
    let status = response.status();
    if !status.is_success() {
        return Err(format!("received HTTP {status} from {url}"));
    }

    response
        .json::<ForkChoiceResponse>()
        .await
        .map(|fork_choice| fork_choice.finalized.slot)
        .map_err(|err| format!("Unable to decode fork_choice response from {url}: {err}"))
}

fn common_non_genesis_finalized_slot(observations: &[FinalizationObservation]) -> Option<u64> {
    let mut slots = observations
        .iter()
        .map(|observation| observation.slot)
        .collect::<Option<Vec<_>>>()?;
    if slots.len() != CLIENT_INTEROP_NODE_COUNT || slots.contains(&0) {
        return None;
    }

    let first_slot = slots.pop()?;
    if slots.iter().all(|slot| *slot == first_slot) {
        Some(first_slot)
    } else {
        None
    }
}

fn format_finalization_observations(observations: &[FinalizationObservation]) -> String {
    if observations.is_empty() {
        return "none".to_string();
    }

    observations
        .iter()
        .map(|observation| {
            if let Some(slot) = observation.slot {
                format!(
                    "{}({}) finalized_slot={}",
                    observation.node_id, observation.client_kind, slot
                )
            } else {
                format!(
                    "{}({}) error={}",
                    observation.node_id,
                    observation.client_kind,
                    observation
                        .error
                        .as_deref()
                        .unwrap_or("unknown forkchoice error")
                )
            }
        })
        .collect::<Vec<_>>()
        .join("; ")
}

fn outer_timeout_for_genesis(genesis_time: u64) -> Duration {
    Duration::from_secs(
        genesis_time.saturating_sub(current_unix_time())
            + FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS
            + OUTER_TEST_TIMEOUT_GRACE_SECS,
    )
}
