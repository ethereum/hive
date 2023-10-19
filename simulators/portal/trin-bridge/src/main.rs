use ethportal_api::HistoryContentKey;
use ethportal_api::HistoryContentValue;
use ethportal_api::PossibleHistoryContentValue;
use ethportal_api::{Discv5ApiClient, HistoryNetworkApiClient};
use hivesim::{dyn_async, Client, Simulation, Suite, Test, TestSpec, TwoClientTestSpec};
use itertools::Itertools;
use portal_bridge::bridge::Bridge;
use portal_bridge::execution_api::ExecutionApi;
use portal_bridge::mode::BridgeMode;
use portal_bridge::pandaops::PandaOpsMiddleware;
use serde_yaml::Value;
use tokio::time::Duration;
use trin_validation::accumulator::MasterAccumulator;
use trin_validation::oracle::HeaderOracle;

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let mut suite = Suite {
        name: "trin-bridge-tests".to_string(),
        description: "The portal bridge test suite".to_string(),
        tests: vec![],
    };

    suite.add(TestSpec {
        name: "Trin bridge tests".to_string(),
        description: "".to_string(),
        always_run: false,
        run: test_portal_bridge,
        client: None,
    });

    let sim = Simulation::new();
    run_suite(sim, suite).await;
}

async fn run_suite(host: Simulation, suite: Suite) {
    let name = suite.clone().name;
    let description = suite.clone().description;

    let suite_id = host.start_suite(name, description, "".to_string()).await;

    for test in &suite.tests {
        test.run_test(host.clone(), suite_id, suite.clone()).await;
    }

    host.end_suite(suite_id).await;
}

dyn_async! {
   async fn test_portal_bridge<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for (client_a, client_b) in clients.iter().cartesian_product(clients.iter()) {
            test.run(
                TwoClientTestSpec {
                    name: format!("Bridge test. A:{} --> B:{}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_bridge,
                    client_a: client_a.clone(),
                    client_b: client_b.clone(),
                }
            ).await;
        }
   }
}

dyn_async! {
    async fn test_bridge<'a> (client_a: Client, client_b: Client) {
        let master_acc = MasterAccumulator::default();
        let header_oracle = HeaderOracle::new(master_acc);
        let portal_clients = vec![client_a.rpc.clone()];
        let epoch_acc_path = "validation_assets/epoch_acc.bin".into();
        let mode = BridgeMode::Test("./test-data/test_data_collection_of_forks_blocks.yaml".into());
        let pandaops_middleware = PandaOpsMiddleware::default();
        let execution_api = ExecutionApi::new(pandaops_middleware);

        // connect clients
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };
        match HistoryNetworkApiClient::add_enr(&client_a.rpc, client_b_enr.clone()).await {
            Ok(response) => if !response {
                panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // start the bridge
        let bridge = Bridge::new(
            mode,
            execution_api,
            portal_clients,
            header_oracle,
            epoch_acc_path,
        );
        bridge.launch().await;

        // wait 2 seconds for data to propagate
        // This value is determined by how long the sleeps are in the bridge code
        // So we may lower this or remove it depending on what we find.
        tokio::time::sleep(Duration::from_secs(2)).await;

        // With default node settings nodes should be storing all content
        let values = std::fs::read_to_string("./test-data/test_data_collection_of_forks_blocks.yaml")
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let comments = vec!["1 header", "1 block body", "100 header",
            "100 block body", "7000000 header", "7000000 block body",
            "7000000 receipt", "15600000 (post-merge) header", "15600000 (post-merge) block body", "15600000 (post-merge) receipt",
            "17510000 (post-shanghai) header", "17510000 (post-shanghai) block body", "17510000 (post-shanghai) receipt"];

        let mut result = vec![];
        for (index, value) in values.as_sequence().unwrap().iter().enumerate() {
            let content_key: HistoryContentKey =
                serde_yaml::from_value(value.get("content_key").unwrap().clone()).unwrap();
            let content_value: HistoryContentValue =
                serde_yaml::from_value(value.get("content_value").unwrap().clone()).unwrap();

            match client_b.rpc.local_content(content_key.clone()).await {
                Ok(possible_content) => {
                   match possible_content {
                        PossibleHistoryContentValue::ContentPresent(content) => {
                            if content != content_value {
                                result.push(format!("Error content received for block {} was different then expected", comments[index]));
                            }
                        }
                        PossibleHistoryContentValue::ContentAbsent => {
                            result.push(format!("Error content for block {} was absent", comments[index]));
                        }
                    }
                }
                Err(err) => {
                    panic!("Unable to get received content: {err:?}");
                }
            }
        }

        if !result.is_empty() {
            panic!("Client B: {:?}", result);
        }
    }
}
