use ethportal_api::types::portal::ContentInfo;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{
    ContentValue, Discv5ApiClient, HistoryContentKey, HistoryContentValue, HistoryNetworkApiClient,
    OverlayContentKey, PossibleHistoryContentValue,
};
use hivesim::{
    dyn_async, Client, NClientTestSpec, Simulation, Suite, Test, TestSpec, TwoClientTestSpec,
};
use itertools::Itertools;
use serde_json::json;
use serde_yaml::Value;
use tokio::time::Duration;

// This is taken from Trin. It should be fairly standard
const MAX_PORTAL_CONTENT_PAYLOAD_SIZE: usize = 1165;

// Header with proof for block number 14764013
const HEADER_WITH_PROOF_KEY: &str =
    "0x00720704f3aa11c53cf344ea069db95cecb81ad7453c8f276b2a1062979611f09c";

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let mut suite = Suite {
        name: "portal-interop".to_string(),
        description:
            "The portal portal-interop test suite runs a set of scenarios to test interoperability between
        portal network clients"
                .to_string(),
        tests: vec![],
    };

    suite.add(TestSpec {
        name: "Portal Network portal-interop".to_string(),
        description: "".to_string(),
        always_run: false,
        run: test_portal_interop,
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

fn content_pair_to_string_pair(
    content_pair: (HistoryContentKey, HistoryContentValue),
) -> (String, String) {
    let (content_key, content_value) = content_pair;
    (content_key.to_hex(), hex_encode(content_value.encode()))
}

struct ProcessedContent {
    content_type: String,
    block_number: u64,
    test_data: Vec<(String, String)>,
}

fn process_content(
    content: Vec<(HistoryContentKey, HistoryContentValue)>,
) -> Vec<ProcessedContent> {
    let mut last_header = content.get(0).unwrap().clone();

    let mut result: Vec<ProcessedContent> = vec![];
    for history_content in content.into_iter() {
        if let HistoryContentKey::BlockHeaderWithProof(_) = &history_content.0 {
            last_header = history_content.clone();
        }
        let (content_type, block_number, test_data) =
            if let HistoryContentValue::BlockHeaderWithProof(header_with_proof) = &last_header.1 {
                match &history_content.0 {
                    HistoryContentKey::BlockHeaderWithProof(_) => (
                        "Block Header".to_string(),
                        header_with_proof.header.number,
                        vec![content_pair_to_string_pair(last_header.clone())],
                    ),
                    HistoryContentKey::BlockBody(_) => (
                        "Block Body".to_string(),
                        header_with_proof.header.number,
                        vec![
                            content_pair_to_string_pair(last_header.clone()),
                            content_pair_to_string_pair(history_content),
                        ],
                    ),
                    HistoryContentKey::BlockReceipts(_) => (
                        "Block Receipt".to_string(),
                        header_with_proof.header.number,
                        vec![
                            content_pair_to_string_pair(last_header.clone()),
                            content_pair_to_string_pair(history_content),
                        ],
                    ),
                    HistoryContentKey::EpochAccumulator(_) => (
                        "Epoch Accumulator".to_string(),
                        header_with_proof.header.number,
                        vec![],
                    ),
                }
            } else {
                unreachable!("History test dated is formatted incorrectly")
            };
        result.push(ProcessedContent {
            content_type,
            block_number,
            test_data,
        })
    }
    result
}

fn get_flair(block_number: u64) -> String {
    if block_number > 17034870 {
        " (post-shanghai)".to_string()
    } else if block_number > 15537394 {
        " (post-merge)".to_string()
    } else if block_number > 12965000 {
        " (post-london)".to_string()
    } else if block_number > 12244000 {
        " (post-berlin)".to_string()
    } else if block_number > 9069000 {
        " (post-istanbul)".to_string()
    } else if block_number > 7280000 {
        " (post-constantinople)".to_string()
    } else if block_number > 4370000 {
        " (post-byzantium)".to_string()
    } else if block_number > 1150000 {
        " (post-homestead)".to_string()
    } else {
        "".to_string()
    }
}

dyn_async! {
   async fn test_portal_interop<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;

        let values = std::fs::read_to_string("./test-data/test_data_collection_of_forks_blocks.yaml")
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let content: Vec<(HistoryContentKey, HistoryContentValue)> = values.as_sequence().unwrap().iter().map(|value| {
            let header_key: HistoryContentKey =
                serde_yaml::from_value(value.get("content_key").unwrap().clone()).unwrap();
            let header_value: HistoryContentValue =
                serde_yaml::from_value(value.get("content_value").unwrap().clone()).unwrap();
            (header_key, header_value)
        }).collect();

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for (client_a, client_b) in clients.iter().cartesian_product(clients.iter()) {
            for ProcessedContent { content_type, block_number, test_data } in process_content(content.clone()) {
                test.run(
                    NClientTestSpec {
                        name: format!("OFFER {}: block number {}{} {} --> {}", content_type, block_number, get_flair(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_offer_x,
                        environment: None,
                        test_data: Some(test_data),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            // Test portal history ping
            test.run(TwoClientTestSpec {
                    name: format!("PING {} --> {}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_ping,
                    client_a: client_a.clone(),
                    client_b: client_b.clone(),
                }
            ).await;

            // Test find content non-present
            test.run(TwoClientTestSpec {
                    name: format!("FIND_CONTENT non present {} --> {}", client_a.name, client_b.name),
                    description: "find content: calls find content that doesn't exist".to_string(),
                    always_run: false,
                    run: test_find_content_non_present,
                    client_a: client_a.clone(),
                    client_b: client_b.clone(),
                }
            ).await;

            // Test find nodes distance zero
            test.run(TwoClientTestSpec {
                    name: format!("FIND_NODES Distance 0 {} --> {}", client_a.name, client_b.name),
                    description: "find nodes: distance zero expect called nodes enr".to_string(),
                    always_run: false,
                    run: test_find_nodes_zero_distance,
                    client_a: client_a.clone(),
                    client_b: client_b.clone(),
                }
            ).await;

            for ProcessedContent { content_type, block_number, test_data } in process_content(content.clone()) {
                test.run(
                    NClientTestSpec {
                        name: format!("RecursiveFindContent {}: block number {}{} {} --> {}", content_type, block_number, get_flair(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_recursive_find_content_x,
                        environment: None,
                        test_data: Some(test_data),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            for ProcessedContent { content_type, block_number, test_data } in process_content(content.clone()) {
                test.run(
                    NClientTestSpec {
                        name: format!("FindContent {}: block number {}{} {} --> {}", content_type, block_number, get_flair(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_find_content_x,
                        environment: None,
                        test_data: Some(test_data),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            // Test gossiping a collection of blocks to node B (B will gossip back to A)
            test.run(
                NClientTestSpec {
                    name: format!("GOSSIP blocks from A:{} --> B:{}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_gossip_two_nodes,
                    environment: None,
                    test_data: Some(content.clone().into_iter().map(content_pair_to_string_pair).collect()),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    // test that a node will not return content via FINDCONTENT.
    async fn test_find_content_non_present<'a> (client_a: Client, client_b: Client) {
        let header_with_proof_key: HistoryContentKey = serde_json::from_value(json!(HEADER_WITH_PROOF_KEY)).unwrap();

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        let result = client_a.rpc.find_content(target_enr, header_with_proof_key.clone()).await;

        match result {
            Ok(result) => {
                match result {
                    ContentInfo::Enrs{ enrs: val } => {
                        if !val.is_empty() {
                            panic!("Error: Unexpected FINDCONTENT response: expected ContentInfo::Enrs length 0 got {}", val.len());
                        }
                    },
                    ContentInfo::Content{ content: _, .. } => {
                        panic!("Error: Unexpected FINDCONTENT response: wasn't supposed to return back content");
                    },
                    other => {
                        panic!("Error: Unexpected FINDCONTENT response: {other:?}");
                    }
                }
            },
            Err(err) => {
                panic!("Error: Unable to get response from FINDCONTENT request: {err:?}");
            }
        }
    }
}

dyn_async! {
    async fn test_offer_x<'a>(clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };
        if let Some((optional_key, optional_value)) = test_data.get(1) {
            let optional_key: HistoryContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: HistoryContentValue =
                serde_json::from_value(json!(optional_value)).unwrap();
            match client_b.rpc.store(optional_key, optional_value).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for recursive find content");
                },
                Err(err) => {
                    panic!("Error storing optional content for recursive find content: {err:?}");
                }
            }
        }
        let (target_key, target_value) = test_data.get(0).expect("Target content is required for this test");
        let target_key: HistoryContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: HistoryContentValue =
            serde_json::from_value(json!(target_value)).unwrap();
        match client_b.rpc.store(target_key.clone(), target_value.clone()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for recursive find content");
            },
            Err(err) => {
                panic!("Error storing target content: {err:?}");
            }
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        let _ = client_a.rpc.offer(target_enr, target_key.clone(), Some(target_value.clone())).await;

        tokio::time::sleep(Duration::from_secs(8)).await;

        match client_b.rpc.local_content(target_key).await {
            Ok(possible_content) => {
               match possible_content {
                    PossibleHistoryContentValue::ContentPresent(content) => {
                        if content != target_value {
                            panic!("Error receiving content: Expected content: {target_value:?}, Received content: {content:?}");
                        }
                    }
                    PossibleHistoryContentValue::ContentAbsent => {
                        panic!("Expected content not found!");
                    }
                }
            }
            Err(err) => {
                panic!("Unable to get received content: {err:?}");
            }
        }
    }
}

dyn_async! {
    async fn test_ping<'a>(client_a: Client, client_b: Client) {
        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        let pong = client_a.rpc.ping(target_enr).await;

        if let Err(err) = pong {
                panic!("Unable to receive pong info: {err:?}");
        }

        // Verify that client_b stored client_a its ENR through the base layer
        // handshake mechanism.
        let stored_enr = match client_a.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        match HistoryNetworkApiClient::get_enr(&client_b.rpc, stored_enr.node_id()).await {
            Ok(response) => {
                if response != stored_enr {
                    panic!("Response from GetEnr didn't return expected ENR. Got: {response}; Expected: {stored_enr}")
                }
            },
            Err(err) => panic!("Failed while trying to get client A's ENR from client B: {err}"),
        }
    }
}

dyn_async! {
    async fn test_find_nodes_zero_distance<'a>(client_a: Client, client_b: Client) {
        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        match client_a.rpc.find_nodes(target_enr.clone(), vec![0]).await {
            Ok(response) => {
                if response.len() != 1 {
                    panic!("Response from FindNodes didn't return expected length of 1");
                }

                match response.get(0) {
                    Some(response_enr) => {
                        if *response_enr != target_enr {
                            panic!("Response from FindNodes didn't return expected Enr");
                        }
                    },
                    None => panic!("Error find nodes zero distance wasn't supposed to return None"),
                }
            }
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    // test that a node will return a content via RECURSIVEFINDCONTENT template that it has stored locally
    async fn test_recursive_find_content_x<'a>(clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };
        if let Some((optional_key, optional_value)) = test_data.get(1) {
            let optional_key: HistoryContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: HistoryContentValue =
                serde_json::from_value(json!(optional_value)).unwrap();
            match client_b.rpc.store(optional_key, optional_value).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for recursive find content");
                },
                Err(err) => {
                    panic!("Error storing optional content for recursive find content: {err:?}");
                }
            }
        }

        let (target_key, target_value) = test_data.get(0).expect("Target content is required for this test");
        let target_key: HistoryContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: HistoryContentValue =
            serde_json::from_value(json!(target_value)).unwrap();
        match client_b.rpc.store(target_key.clone(), target_value.clone()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for recursive find content");
            },
            Err(err) => {
                panic!("Error storing target content: {err:?}");
            }
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        match HistoryNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        match client_a.rpc.recursive_find_content(target_key.clone()).await {
            Ok(result) => {
                match result {
                    ContentInfo::Content{ content: ethportal_api::PossibleHistoryContentValue::ContentPresent(val), utp_transfer } => {
                        if val != target_value {
                            panic!("Error: Unexpected RECURSIVEFINDCONTENT response: didn't return expected target content");
                        }

                        if target_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
                            if utp_transfer {
                                panic!("Error: Unexpected RECURSIVEFINDCONTENT response: utp_transfer was supposed to be false");
                            }
                        } else if !utp_transfer {
                            panic!("Error: Unexpected RECURSIVEFINDCONTENT response: utp_transfer was supposed to be true");
                        }
                    },
                    other => {
                        panic!("Error: Unexpected RECURSIVEFINDCONTENT response: {other:?}");
                    }
                }
            },
            Err(err) => {
                panic!("Error: Unable to get response from RECURSIVEFINDCONTENT request: {err:?}");
            }
        }
    }
}

dyn_async! {
    // test that a node will return a x content via FINDCONTENT that it has stored locally
    async fn test_find_content_x<'a> (clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };
        if let Some((optional_key, optional_value)) = test_data.get(1) {
            let optional_key: HistoryContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: HistoryContentValue =
                serde_json::from_value(json!(optional_value)).unwrap();
            match client_b.rpc.store(optional_key, optional_value).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for find content");
                },
                Err(err) => {
                    panic!("Error storing optional content for find content: {err:?}");
                }
            }
        }

        let (target_key, target_value) = test_data.get(0).expect("Target content is required for this test");
        let target_key: HistoryContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: HistoryContentValue =
            serde_json::from_value(json!(target_value)).unwrap();
        match client_b.rpc.store(target_key.clone(), target_value.clone()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for find content");
            },
            Err(err) => {
                panic!("Error storing target content: {err:?}");
            }
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        match client_a.rpc.find_content(target_enr, target_key.clone()).await {
            Ok(result) => {
                match result {
                    ContentInfo::Content{ content: ethportal_api::PossibleHistoryContentValue::ContentPresent(val), utp_transfer } => {
                        if val != target_value {
                            panic!("Error: Unexpected FINDCONTENT response: didn't return expected block body");
                        }

                        if target_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
                            if utp_transfer {
                                panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be false");
                            }
                        } else if !utp_transfer {
                            panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be true");
                        }
                    },
                    other => {
                        panic!("Error: Unexpected FINDCONTENT response: {other:?}");
                    }
                }
            },
            Err(err) => {
                panic!("Error: Unable to get response from FINDCONTENT request: {err:?}");
            }
        }
    }
}

dyn_async! {
    async fn test_gossip_two_nodes<'a> (clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };
        // connect clients
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };
        match HistoryNetworkApiClient::add_enr(&client_a.rpc, client_b_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // With default node settings nodes should be storing all content
        for (content_key, content_value) in test_data.clone() {
            let content_key: HistoryContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_value: HistoryContentValue =
                serde_json::from_value(json!(content_value)).unwrap();

            match client_a.rpc.gossip(content_key.clone(), content_value.clone()).await {
                Ok(nodes_gossiped_to) => {
                   if nodes_gossiped_to != 1 {
                        panic!("We expected to gossip to 1 node instead we gossiped to: {nodes_gossiped_to}");
                    }
                }
                Err(err) => {
                    panic!("Unable to get received content: {err:?}");
                }
            }

            if let HistoryContentKey::BlockHeaderWithProof(_) = content_key {
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
        }

        // wait 8 seconds for data to propagate
        // This value is determined by how long the sleeps are in the gossip code
        // So we may lower this or remove it depending on what we find.
        tokio::time::sleep(Duration::from_secs(8)).await;

        let comments = vec!["1 header", "1 block body", "100 header",
            "100 block body", "7000000 header", "7000000 block body",
            "7000000 receipt", "15600000 (post-merge) header", "15600000 (post-merge) block body", "15600000 (post-merge) receipt",
            "17510000 (post-shanghai) header", "17510000 (post-shanghai) block body", "17510000 (post-shanghai) receipt"];

        let mut result = vec![];
        for (index, (content_key, content_value)) in test_data.into_iter().enumerate() {
            let content_key: HistoryContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_value: HistoryContentValue =
                serde_json::from_value(json!(content_value)).unwrap();

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
