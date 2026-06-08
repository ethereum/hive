use crate::utils::util::{
    bootnode_metadata_for_client, current_unix_time, default_genesis_time, http_client,
    lean_api_url, lean_bootnodes_for_client, lean_client_kind, lean_clients, lean_environment,
    panic_payload_to_string, prepare_client_runtime_files, run_data_test_with_timeout,
    CheckpointResponse, ForkChoiceResponse, LeanBootnodeMetadata, TimedDataTestSpec,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, Test};
use std::collections::{HashMap, HashSet};
use std::time::Duration;
use tokio::time::{sleep, timeout};

const BOOTNODES_ENVIRONMENT_VARIABLE: &str = "HIVE_BOOTNODES";
const AGGREGATE_SUBNET_IDS_ENVIRONMENT_VARIABLE: &str = "HIVE_AGGREGATE_SUBNET_IDS";
const CLIENT_PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";
const IS_AGGREGATOR_ENVIRONMENT_VARIABLE: &str = "HIVE_IS_AGGREGATOR";
const LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE: &str =
    "HIVE_ATTESTATION_COMMITTEE_COUNT";
const LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_GENESIS_TIME";
const LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_COUNT";
const LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE: &str = "HIVE_VALIDATOR_INDEX";
const LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE: &str = "HIVE_LEAN_VALIDATOR_INDICES";
const NODE_ID_ENVIRONMENT_VARIABLE: &str = "HIVE_NODE_ID";

const CLIENT_INTEROP_P2P_PORT: u16 = 9000;
const SINGLE_SUBNET_ATTESTATION_COMMITTEE_COUNT: usize = 1;
const TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT: usize = 2;
const SINGLE_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS: u64 = 3 * 60;
const TWO_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS: u64 = 210; //~50 slots
const TWO_SUBNET_GENESIS_DELAY_SECS: u64 = 60;
const CLIENT_STARTUP_ATTEMPTS: u64 = 3;
const CLIENT_STARTUP_TIMEOUT_SECS: u64 = 120;
const OUTER_TEST_TIMEOUT_GRACE_SECS: u64 = 750;
const FINALIZATION_POLL_INTERVAL_SECS: u64 = 1;

#[derive(Clone)]
struct ClientInteropNode {
    client: ClientDefinition,
    client_kind: String,
    node_id: String,
    validator_indices: Vec<usize>,
    private_key: String,
    is_aggregator: bool,
}

#[derive(Clone)]
struct ClientInteropTestData {
    run_label: String,
    nodes: Vec<ClientInteropNode>,
    genesis_time: u64,
    genesis_validator_count: usize,
    attestation_committee_count: usize,
    finalization_timeout_after_genesis_secs: u64,
}

struct ClientInteropTopologySpec {
    left_name: String,
    right_name: String,
    topology: Vec<ClientDefinition>,
}

#[derive(Clone, Copy)]
enum AggregatorPlacement {
    Majority,
    Minority,
}

impl AggregatorPlacement {
    fn label(self) -> &'static str {
        match self {
            AggregatorPlacement::Majority => "majority aggregator",
            AggregatorPlacement::Minority => "minority aggregator",
        }
    }

    fn single_subnet_description(self) -> &'static str {
        match self {
            AggregatorPlacement::Majority => "one majority node",
            AggregatorPlacement::Minority => "the minority node, or the first node for self tests",
        }
    }

    fn two_subnet_description(self) -> &'static str {
        match self {
            AggregatorPlacement::Majority => "one majority node in each subnet",
            AggregatorPlacement::Minority => {
                "the minority node in each subnet, or the first node in each subnet for self tests"
            }
        }
    }
}

struct RunningInteropClient {
    node_id: String,
    client_kind: String,
    client: Client,
}

struct FinalizationObservation {
    node_id: String,
    client_kind: String,
    checkpoint: Option<CheckpointResponse>,
    error: Option<String>,
}

dyn_async! {
    pub async fn run_client_interop_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        assert!(!clients.is_empty(), "client-interop requires at least one selected lean client, got {}", clients.len());

        for aggregator_placement in aggregator_placements() {
            for topology_spec in interop_topology_matrix(&clients) {
                let mut nodes = build_interop_nodes(topology_spec.topology);
                assign_single_subnet_aggregator(&mut nodes, aggregator_placement);

                let genesis_time = default_genesis_time();
                let topology_label = topology_label(&nodes);
                let run_label = format!(
                    "{} {} and {} / {}",
                    aggregator_placement.label(),
                    topology_spec.left_name,
                    topology_spec.right_name,
                    topology_label
                );

                run_data_test_with_timeout(
                    test,
                    TimedDataTestSpec {
                        name: format!("client-interop: {run_label}"),
                        description: format!(
                            "Starts {topology_label} with a shared genesis, sets {} as aggregator, and checks that all three nodes finalize past genesis at the same checkpoint.",
                            aggregator_placement.single_subnet_description()
                        ),
                        always_run: false,
                        client_name: run_label.clone(),
                        timeout_duration: outer_timeout_for_genesis(
                            genesis_time,
                            SINGLE_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
                        ),
                        test_data: ClientInteropTestData {
                            run_label,
                            genesis_validator_count: validator_count_for_nodes(&nodes),
                            attestation_committee_count: SINGLE_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                            finalization_timeout_after_genesis_secs:
                                SINGLE_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
                            nodes,
                            genesis_time,
                        },
                    },
                    test_client_interop_finalizes,
                )
                .await;
            }
        }

        for aggregator_placement in aggregator_placements() {
            for topology_spec in two_subnet_interop_topology_matrix(&clients) {
                let mut nodes = build_two_subnet_interop_nodes(&topology_spec);
                assign_two_subnet_aggregators(&mut nodes, aggregator_placement);

                let genesis_time = two_subnet_genesis_time();
                let topology_label = topology_label(&nodes);
                let run_label = format!(
                    "two-subnet {} {} minority and {} majority / {}",
                    aggregator_placement.label(),
                    topology_spec.left_name,
                    topology_spec.right_name,
                    topology_label
                );

                run_data_test_with_timeout(
                    test,
                    TimedDataTestSpec {
                        name: format!("client-interop: {run_label}"),
                        description: format!(
                            "Starts {topology_label} across two attestation subnets with a shared genesis, sets {} as aggregator, and checks that all nodes finalize past genesis at the same checkpoint.",
                            aggregator_placement.two_subnet_description()
                        ),
                        always_run: false,
                        client_name: run_label.clone(),
                        timeout_duration: outer_timeout_for_genesis(
                            genesis_time,
                            TWO_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
                        ),
                        test_data: ClientInteropTestData {
                            run_label,
                            genesis_validator_count: validator_count_for_nodes(&nodes),
                            attestation_committee_count: TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                            finalization_timeout_after_genesis_secs:
                                TWO_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
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
}

dyn_async! {
    async fn test_client_interop_finalizes<'a>(test: &'a mut Test, test_data: ClientInteropTestData) {
        let mut running_clients = Vec::with_capacity(test_data.nodes.len());
        let mut bootnodes = Vec::with_capacity(test_data.nodes.len());

        for node in &test_data.nodes {
            let client = start_interop_client(test, node, &bootnodes, &test_data).await;
            let metadata =
                bootnode_metadata_for_client(&node.private_key, client.ip, CLIENT_INTEROP_P2P_PORT);
            bootnodes.push(metadata);
            running_clients.push(RunningInteropClient {
                node_id: node.node_id.clone(),
                client_kind: node.client_kind.clone(),
                client,
            });
        }

        wait_for_same_non_genesis_finalized_checkpoint(
            &test_data.run_label,
            test_data.genesis_time,
            test_data.finalization_timeout_after_genesis_secs,
            &running_clients,
        )
        .await;
    }
}

fn aggregator_placements() -> [AggregatorPlacement; 2] {
    [AggregatorPlacement::Majority, AggregatorPlacement::Minority]
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

fn two_subnet_interop_topology_matrix(
    clients: &[ClientDefinition],
) -> Vec<ClientInteropTopologySpec> {
    let mut topology_specs = Vec::new();
    let mut self_tested = HashSet::new();

    for left_index in 0..clients.len() {
        for right_index in left_index..clients.len() {
            let left = &clients[left_index];
            let right = &clients[right_index];

            if left.name == right.name && !self_tested.insert(left.name.clone()) {
                continue;
            }

            topology_specs.extend(two_subnet_interop_topologies(left, right).into_iter().map(
                |topology| {
                    let minority_name = topology[0].name.clone();
                    let majority_name = topology[1].name.clone();
                    ClientInteropTopologySpec {
                        left_name: minority_name,
                        right_name: majority_name,
                        topology,
                    }
                },
            ));
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

fn two_subnet_interop_topologies(
    left: &ClientDefinition,
    right: &ClientDefinition,
) -> Vec<Vec<ClientDefinition>> {
    if left.name == right.name {
        return vec![vec![left.clone(), left.clone()]];
    }

    vec![
        vec![left.clone(), right.clone()],
        vec![right.clone(), left.clone()],
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
                validator_indices: vec![validator_index],
                private_key: private_key_for_node(validator_index),
                is_aggregator: false,
            }
        })
        .collect()
}

fn build_two_subnet_interop_nodes(
    topology_spec: &ClientInteropTopologySpec,
) -> Vec<ClientInteropNode> {
    assert_eq!(
        topology_spec.topology.len(),
        2,
        "two-subnet client-interop topology must contain exactly two client definitions"
    );

    let client_a = &topology_spec.topology[0];
    let client_b = &topology_spec.topology[1];

    let topology = [
        (client_a.clone(), vec![0]),
        (client_b.clone(), vec![2]),
        (client_b.clone(), vec![4]),
        (client_a.clone(), vec![1]),
        (client_b.clone(), vec![3]),
        (client_b.clone(), vec![5]),
    ];

    let mut client_kind_counts = HashMap::<String, usize>::new();
    topology
        .into_iter()
        .enumerate()
        .map(|(node_index, (client, validator_indices))| {
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
                validator_indices,
                private_key: private_key_for_node(node_index),
                is_aggregator: false,
            }
        })
        .collect()
}

fn assign_single_subnet_aggregator(
    nodes: &mut [ClientInteropNode],
    placement: AggregatorPlacement,
) {
    let candidate_indices = (0..nodes.len()).collect::<Vec<_>>();
    let aggregator_index = aggregator_index_by_client_count(nodes, &candidate_indices, placement);

    for (index, node) in nodes.iter_mut().enumerate() {
        node.is_aggregator = index == aggregator_index;
    }
}

fn aggregator_index_by_client_count(
    nodes: &[ClientInteropNode],
    candidate_indices: &[usize],
    placement: AggregatorPlacement,
) -> usize {
    assert!(
        !candidate_indices.is_empty(),
        "client-interop aggregator selection requires at least one candidate node"
    );

    let mut client_counts = HashMap::<String, usize>::new();
    for index in candidate_indices {
        let node = &nodes[*index];
        *client_counts.entry(node.client.name.clone()).or_insert(0) += 1;
    }

    let target_count = match placement {
        AggregatorPlacement::Majority => client_counts
            .values()
            .max()
            .copied()
            .expect("client-interop topology should have client counts"),
        AggregatorPlacement::Minority => client_counts
            .values()
            .min()
            .copied()
            .expect("client-interop topology should have client counts"),
    };

    candidate_indices
        .iter()
        .copied()
        .find(|index| client_counts[&nodes[*index].client.name] == target_count)
        .expect("client-interop topology should include the target aggregator")
}

fn assign_two_subnet_aggregators(nodes: &mut [ClientInteropNode], placement: AggregatorPlacement) {
    for node in nodes.iter_mut() {
        node.is_aggregator = false;
    }

    for subnet_id in 0..TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT {
        let candidate_indices = nodes
            .iter()
            .enumerate()
            .filter(|(_, node)| {
                node_has_validator_in_subnet(
                    node,
                    subnet_id,
                    TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                )
            })
            .map(|(index, _)| index)
            .collect::<Vec<_>>();
        let aggregator_index =
            aggregator_index_by_client_count(nodes, &candidate_indices, placement);
        nodes[aggregator_index].is_aggregator = true;
    }
}

fn subnet_id_for_validator_index(
    validator_index: usize,
    attestation_committee_count: usize,
) -> usize {
    validator_index % attestation_committee_count
}

fn node_has_validator_in_subnet(
    node: &ClientInteropNode,
    subnet_id: usize,
    attestation_committee_count: usize,
) -> bool {
    node.validator_indices.iter().any(|index| {
        subnet_id_for_validator_index(*index, attestation_committee_count) == subnet_id
    })
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

fn validator_count_for_nodes(nodes: &[ClientInteropNode]) -> usize {
    nodes
        .iter()
        .flat_map(|node| node.validator_indices.iter())
        .max()
        .map(|index| index + 1)
        .unwrap_or(0)
}

async fn start_interop_client(
    test: &Test,
    node: &ClientInteropNode,
    bootnodes: &[LeanBootnodeMetadata],
    test_data: &ClientInteropTestData,
) -> Client {
    let environment = client_interop_environment(node, bootnodes, test_data);
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
                "startup attempt exceeded {CLIENT_STARTUP_TIMEOUT_SECS} seconds"
            ))
        }
    }
}

fn client_interop_environment(
    node: &ClientInteropNode,
    bootnodes: &[LeanBootnodeMetadata],
    test_data: &ClientInteropTestData,
) -> HashMap<String, String> {
    let mut environment = lean_environment();
    let validator_indices = validator_indices_csv(&node.validator_indices);

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
        test_data.attestation_committee_count.to_string(),
    );
    environment.insert(
        LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE.to_string(),
        test_data.genesis_validator_count.to_string(),
    );
    environment.insert(
        LEAN_GENESIS_TIME_ENVIRONMENT_VARIABLE.to_string(),
        test_data.genesis_time.to_string(),
    );
    environment.insert(
        LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE.to_string(),
        node.validator_indices
            .first()
            .expect("client-interop nodes must have at least one validator index")
            .to_string(),
    );
    environment.insert(
        LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE.to_string(),
        validator_indices,
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
        if test_data.attestation_committee_count > 1 {
            environment.insert(
                AGGREGATE_SUBNET_IDS_ENVIRONMENT_VARIABLE.to_string(),
                aggregate_subnet_ids_csv(test_data.attestation_committee_count),
            );
        }
    }

    environment
}

fn aggregate_subnet_ids_csv(attestation_committee_count: usize) -> String {
    (0..attestation_committee_count)
        .map(|index| index.to_string())
        .collect::<Vec<_>>()
        .join(",")
}

fn validator_indices_csv(indices: &[usize]) -> String {
    indices
        .iter()
        .map(|index| index.to_string())
        .collect::<Vec<_>>()
        .join(",")
}

async fn wait_for_same_non_genesis_finalized_checkpoint(
    run_label: &str,
    genesis_time: u64,
    finalization_timeout_after_genesis_secs: u64,
    clients: &[RunningInteropClient],
) {
    let deadline = genesis_time + finalization_timeout_after_genesis_secs;
    let mut last_observations = Vec::new();

    while current_unix_time() <= deadline {
        let observations = load_finalization_observations(clients).await;
        if let Some(finalized_checkpoint) = common_non_genesis_finalized_checkpoint(&observations) {
            assert!(
                finalized_checkpoint.slot > 0,
                "client-interop finalized checkpoint should be past genesis"
            );
            return;
        }

        last_observations = observations;
        sleep(Duration::from_secs(FINALIZATION_POLL_INTERVAL_SECS)).await;
    }

    panic!(
        "client-interop run {} did not observe all nodes finalized past genesis at the same checkpoint within {} seconds after genesis_time {} (deadline {}). Last observations: {}",
        run_label,
        finalization_timeout_after_genesis_secs,
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
        match try_load_finalized_checkpoint(&running_client.client).await {
            Ok(checkpoint) => observations.push(FinalizationObservation {
                node_id: running_client.node_id.clone(),
                client_kind: running_client.client_kind.clone(),
                checkpoint: Some(checkpoint),
                error: None,
            }),
            Err(error) => observations.push(FinalizationObservation {
                node_id: running_client.node_id.clone(),
                client_kind: running_client.client_kind.clone(),
                checkpoint: None,
                error: Some(error),
            }),
        }
    }

    observations
}

async fn try_load_finalized_checkpoint(client: &Client) -> Result<CheckpointResponse, String> {
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
        .map(|fork_choice| fork_choice.finalized)
        .map_err(|err| format!("Unable to decode fork_choice response from {url}: {err}"))
}

fn common_non_genesis_finalized_checkpoint(
    observations: &[FinalizationObservation],
) -> Option<&CheckpointResponse> {
    let mut checkpoints = observations
        .iter()
        .map(|observation| observation.checkpoint.as_ref())
        .collect::<Option<Vec<_>>>()?;
    if checkpoints.is_empty() || checkpoints.iter().any(|checkpoint| checkpoint.slot == 0) {
        return None;
    }

    let first_checkpoint = checkpoints.pop()?;
    if checkpoints.iter().all(|checkpoint| {
        checkpoint.slot == first_checkpoint.slot && checkpoint.root == first_checkpoint.root
    }) {
        Some(first_checkpoint)
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
            if let Some(checkpoint) = &observation.checkpoint {
                format!(
                    "{}({}) finalized_slot={} finalized_root={}",
                    observation.node_id, observation.client_kind, checkpoint.slot, checkpoint.root
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

fn outer_timeout_for_genesis(
    genesis_time: u64,
    finalization_timeout_after_genesis_secs: u64,
) -> Duration {
    Duration::from_secs(
        genesis_time.saturating_sub(current_unix_time())
            + finalization_timeout_after_genesis_secs
            + OUTER_TEST_TIMEOUT_GRACE_SECS,
    )
}

fn two_subnet_genesis_time() -> u64 {
    current_unix_time() + TWO_SUBNET_GENESIS_DELAY_SECS
}

#[cfg(test)]
mod tests {
    use super::*;
    use alloy_primitives::B256;
    use hivesim::types::ClientMetadata;

    fn client(name: &str) -> ClientDefinition {
        ClientDefinition {
            name: name.to_string(),
            version: "test".to_string(),
            meta: ClientMetadata { roles: vec![] },
        }
    }

    fn devnet4_lean_clients() -> Vec<ClientDefinition> {
        vec![
            client("ethlambda_devnet4"),
            client("gean_devnet4"),
            client("grandine_lean_devnet4"),
            client("lantern_devnet4"),
            client("ream_devnet4"),
            client("zeam_devnet4"),
            client("qlean_devnet4"),
        ]
    }

    fn finalized_observation(slot: u64, root: B256) -> FinalizationObservation {
        FinalizationObservation {
            node_id: "node".to_string(),
            client_kind: "client".to_string(),
            checkpoint: Some(CheckpointResponse { slot, root }),
            error: None,
        }
    }

    fn aggregator_flags(nodes: &[ClientInteropNode]) -> Vec<bool> {
        nodes.iter().map(|node| node.is_aggregator).collect()
    }

    #[test]
    fn single_subnet_aggregator_placements_select_expected_node() {
        let mut left_majority = build_interop_nodes(vec![
            client("ream_devnet4"),
            client("ream_devnet4"),
            client("gean_devnet4"),
        ]);
        assign_single_subnet_aggregator(&mut left_majority, AggregatorPlacement::Majority);
        assert_eq!(aggregator_flags(&left_majority), vec![true, false, false]);

        assign_single_subnet_aggregator(&mut left_majority, AggregatorPlacement::Minority);
        assert_eq!(aggregator_flags(&left_majority), vec![false, false, true]);

        let mut right_majority = build_interop_nodes(vec![
            client("ream_devnet4"),
            client("gean_devnet4"),
            client("gean_devnet4"),
        ]);
        assign_single_subnet_aggregator(&mut right_majority, AggregatorPlacement::Majority);
        assert_eq!(aggregator_flags(&right_majority), vec![false, true, false]);

        assign_single_subnet_aggregator(&mut right_majority, AggregatorPlacement::Minority);
        assert_eq!(aggregator_flags(&right_majority), vec![true, false, false]);

        let mut self_test = build_interop_nodes(vec![
            client("ream_devnet4"),
            client("ream_devnet4"),
            client("ream_devnet4"),
        ]);
        assign_single_subnet_aggregator(&mut self_test, AggregatorPlacement::Majority);
        assert_eq!(aggregator_flags(&self_test), vec![true, false, false]);

        assign_single_subnet_aggregator(&mut self_test, AggregatorPlacement::Minority);
        assert_eq!(aggregator_flags(&self_test), vec![true, false, false]);
    }

    #[test]
    fn single_subnet_node_builder_accepts_all_devnet4_lean_clients_for_both_placements() {
        for topology_spec in interop_topology_matrix(&devnet4_lean_clients()) {
            for placement in aggregator_placements() {
                let mut nodes = build_interop_nodes(topology_spec.topology.clone());
                assign_single_subnet_aggregator(&mut nodes, placement);

                assert_eq!(nodes.len(), 3);
                assert_eq!(validator_count_for_nodes(&nodes), 3);
                assert_eq!(
                    nodes
                        .iter()
                        .map(|node| node.validator_indices.as_slice())
                        .collect::<Vec<_>>(),
                    vec![&[0][..], &[1][..], &[2][..]]
                );
                assert_eq!(
                    nodes.iter().filter(|node| node.is_aggregator).count(),
                    1,
                    "single-subnet topology should select exactly one aggregator"
                );

                let test_data = ClientInteropTestData {
                    run_label: "single-subnet env".to_string(),
                    genesis_validator_count: validator_count_for_nodes(&nodes),
                    attestation_committee_count: SINGLE_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                    finalization_timeout_after_genesis_secs:
                        SINGLE_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
                    nodes: nodes.clone(),
                    genesis_time: 123,
                };

                for node in &nodes {
                    let environment = client_interop_environment(node, &[], &test_data);
                    assert_eq!(
                        environment[LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE],
                        "1"
                    );
                    assert_eq!(
                        environment[LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE],
                        "3"
                    );
                    assert_eq!(
                        environment[LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE],
                        validator_indices_csv(&node.validator_indices)
                    );
                    assert!(
                        !environment.contains_key(AGGREGATE_SUBNET_IDS_ENVIRONMENT_VARIABLE),
                        "single-subnet tests should not pass aggregate subnet IDs"
                    );

                    if node.is_aggregator {
                        assert_eq!(environment[IS_AGGREGATOR_ENVIRONMENT_VARIABLE], "1");
                    } else {
                        assert!(!environment.contains_key(IS_AGGREGATOR_ENVIRONMENT_VARIABLE));
                    }
                }
            }
        }
    }

    #[test]
    fn two_subnet_node_builder_accepts_all_devnet4_lean_clients() {
        assert_eq!(
            two_subnet_interop_topology_matrix(&devnet4_lean_clients()).len(),
            64
        );
        assert_eq!(aggregator_placements().len(), 2);

        for topology_spec in two_subnet_interop_topology_matrix(&devnet4_lean_clients()) {
            for placement in aggregator_placements() {
                let mut nodes = build_two_subnet_interop_nodes(&topology_spec);
                assign_two_subnet_aggregators(&mut nodes, placement);

                assert_eq!(validator_count_for_nodes(&nodes), 6);
                assert_eq!(
                    nodes
                        .iter()
                        .map(|node| node.validator_indices.as_slice())
                        .collect::<Vec<_>>(),
                    vec![&[0][..], &[2][..], &[4][..], &[1][..], &[3][..], &[5][..]]
                );
                assert_eq!(
                    nodes.iter().filter(|node| node.is_aggregator).count(),
                    TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                    "two-subnet topology should select one aggregator per subnet"
                );

                for subnet_id in 0..TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT {
                    assert_eq!(
                        nodes
                            .iter()
                            .filter(|node| {
                                node.is_aggregator
                                    && node_has_validator_in_subnet(
                                        node,
                                        subnet_id,
                                        TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT,
                                    )
                            })
                            .count(),
                        1,
                        "two-subnet topology should select exactly one aggregator in subnet {subnet_id}"
                    );
                }
            }
        }
    }

    #[test]
    fn two_subnet_nodes_use_one_validator_assignment_per_node() {
        let topology_spec = ClientInteropTopologySpec {
            left_name: "client_a_devnet4".to_string(),
            right_name: "client_b_devnet4".to_string(),
            topology: vec![client("ream_devnet4"), client("gean_devnet4")],
        };

        let mut nodes = build_two_subnet_interop_nodes(&topology_spec);

        let assignments = nodes
            .iter()
            .map(|node| (node.client.name.as_str(), node.validator_indices.as_slice()))
            .collect::<Vec<_>>();
        assert_eq!(
            assignments,
            vec![
                ("ream_devnet4", &[0][..]),
                ("gean_devnet4", &[2][..]),
                ("gean_devnet4", &[4][..]),
                ("ream_devnet4", &[1][..]),
                ("gean_devnet4", &[3][..]),
                ("gean_devnet4", &[5][..]),
            ]
        );

        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Minority);
        assert_eq!(
            nodes
                .iter()
                .map(|node| node.is_aggregator)
                .collect::<Vec<_>>(),
            vec![true, false, false, true, false, false]
        );

        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Majority);
        assert_eq!(
            nodes
                .iter()
                .map(|node| node.is_aggregator)
                .collect::<Vec<_>>(),
            vec![false, true, false, false, true, false]
        );
    }

    #[test]
    fn two_subnet_environment_passes_committee_and_single_validator_assignment() {
        let topology_spec = ClientInteropTopologySpec {
            left_name: "ream_devnet4".to_string(),
            right_name: "gean_devnet4".to_string(),
            topology: vec![client("ream_devnet4"), client("gean_devnet4")],
        };
        let mut nodes = build_two_subnet_interop_nodes(&topology_spec);
        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Minority);
        let test_data = ClientInteropTestData {
            run_label: "two-subnet env".to_string(),
            genesis_validator_count: validator_count_for_nodes(&nodes),
            attestation_committee_count: TWO_SUBNET_ATTESTATION_COMMITTEE_COUNT,
            finalization_timeout_after_genesis_secs:
                TWO_SUBNET_FINALIZATION_TIMEOUT_AFTER_GENESIS_SECS,
            nodes: nodes.clone(),
            genesis_time: 123,
        };

        let environment = client_interop_environment(&nodes[1], &[], &test_data);

        assert_eq!(
            environment[LEAN_ATTESTATION_COMMITTEE_COUNT_ENVIRONMENT_VARIABLE],
            "2"
        );
        assert_eq!(
            environment[LEAN_GENESIS_VALIDATOR_COUNT_ENVIRONMENT_VARIABLE],
            "6"
        );
        assert_eq!(environment[LEAN_VALIDATOR_INDEX_ENVIRONMENT_VARIABLE], "2");
        assert_eq!(
            environment[LEAN_VALIDATOR_INDICES_ENVIRONMENT_VARIABLE],
            "2"
        );
        assert!(!environment.contains_key(IS_AGGREGATOR_ENVIRONMENT_VARIABLE));

        let aggregator_environment = client_interop_environment(&nodes[0], &[], &test_data);
        assert_eq!(
            aggregator_environment[IS_AGGREGATOR_ENVIRONMENT_VARIABLE],
            "1"
        );
        assert_eq!(
            aggregator_environment[AGGREGATE_SUBNET_IDS_ENVIRONMENT_VARIABLE],
            "0,1"
        );

        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Majority);
        let majority_aggregator_environment =
            client_interop_environment(&nodes[1], &[], &test_data);
        assert_eq!(
            majority_aggregator_environment[IS_AGGREGATOR_ENVIRONMENT_VARIABLE],
            "1"
        );
        assert_eq!(
            majority_aggregator_environment[AGGREGATE_SUBNET_IDS_ENVIRONMENT_VARIABLE],
            "0,1"
        );
    }

    #[test]
    fn finalized_checkpoint_agreement_accepts_same_non_genesis_checkpoint() {
        let root = B256::from_slice(&[0xaa; 32]);
        let observations = vec![
            finalized_observation(3, root),
            finalized_observation(3, root),
            finalized_observation(3, root),
        ];

        let checkpoint = common_non_genesis_finalized_checkpoint(&observations)
            .expect("matching non-genesis checkpoints should agree");

        assert_eq!(checkpoint.slot, 3);
        assert_eq!(checkpoint.root, root);
    }

    #[test]
    fn finalized_checkpoint_agreement_rejects_same_slot_with_different_roots() {
        let observations = vec![
            finalized_observation(3, B256::from_slice(&[0xaa; 32])),
            finalized_observation(3, B256::from_slice(&[0xbb; 32])),
        ];

        assert!(common_non_genesis_finalized_checkpoint(&observations).is_none());
    }

    #[test]
    fn finalized_checkpoint_agreement_rejects_genesis_checkpoint() {
        let root = B256::ZERO;
        let observations = vec![
            finalized_observation(0, root),
            finalized_observation(0, root),
        ];

        assert!(common_non_genesis_finalized_checkpoint(&observations).is_none());
    }

    #[test]
    fn two_subnet_same_client_marks_one_node_per_subnet_as_aggregator() {
        let topology_spec = ClientInteropTopologySpec {
            left_name: "ream_devnet4".to_string(),
            right_name: "ream_devnet4".to_string(),
            topology: vec![client("ream_devnet4"), client("ream_devnet4")],
        };

        let mut nodes = build_two_subnet_interop_nodes(&topology_spec);
        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Minority);

        assert_eq!(
            nodes
                .iter()
                .map(|node| node.is_aggregator)
                .collect::<Vec<_>>(),
            vec![true, false, false, true, false, false]
        );

        assign_two_subnet_aggregators(&mut nodes, AggregatorPlacement::Majority);
        assert_eq!(
            nodes
                .iter()
                .map(|node| node.is_aggregator)
                .collect::<Vec<_>>(),
            vec![true, false, false, true, false, false]
        );
    }
}
