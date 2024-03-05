use crate::suites::constants::{
    BEACON_STRING, HIVE_PORTAL_NETWORKS_SELECTED, TEST_DATA_FILE_PATH, TRIN_BRIDGE_CLIENT_TYPE,
};
use ethportal_api::types::beacon::ContentInfo;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{
    BeaconContentKey, BeaconContentValue, BeaconNetworkApiClient, ContentValue, Discv5ApiClient,
    OverlayContentKey,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use serde_json::json;
use serde_yaml::Value;
use std::collections::HashMap;
use tokio::time::Duration;

// This is taken from Trin. It should be fairly standard
const MAX_PORTAL_CONTENT_PAYLOAD_SIZE: usize = 1165;

// Header with proof for block number 14764013
const BOOTSTRAP_KEY: &str = "0x10bd9f42d9a42d972bdaf4dee84e5b419dd432b52867258acb7bcc7f567b6e3af1";

fn content_pair_to_string_pair(
    content_pair: (BeaconContentKey, BeaconContentValue),
) -> (String, String) {
    let (content_key, content_value) = content_pair;
    (content_key.to_hex(), hex_encode(content_value.encode()))
}

/// Processed content data for beacon tests
struct ProcessedContent {
    content_type: String,
    identifier: String,
    test_data: Vec<(String, String)>,
}

fn process_content(content: Vec<(BeaconContentKey, BeaconContentValue)>) -> Vec<ProcessedContent> {
    let mut result: Vec<ProcessedContent> = vec![];
    for beacon_content in content.into_iter() {
        let (content_type, identifier, test_data) = match &beacon_content.0 {
            BeaconContentKey::LightClientBootstrap(bootstrap) => (
                "Bootstrap".to_string(),
                hex_encode(bootstrap.block_hash),
                vec![content_pair_to_string_pair(beacon_content)],
            ),
            BeaconContentKey::LightClientUpdatesByRange(updates_by_range) => (
                "Updates by Range".to_string(),
                format!(
                    "start period: {} count: {}",
                    updates_by_range.start_period, updates_by_range.count
                ),
                vec![content_pair_to_string_pair(beacon_content)],
            ),
            BeaconContentKey::LightClientFinalityUpdate(finality_update) => (
                "Finality Update".to_string(),
                format!("finalized slot: {}", finality_update.finalized_slot),
                vec![content_pair_to_string_pair(beacon_content)],
            ),
            BeaconContentKey::LightClientOptimisticUpdate(optimistic_update) => (
                "Optimistic Update".to_string(),
                format!("finalized slot: {}", optimistic_update.signature_slot),
                vec![content_pair_to_string_pair(beacon_content)],
            ),
        };
        result.push(ProcessedContent {
            content_type,
            identifier,
            test_data,
        })
    }
    result
}

dyn_async! {
   pub async fn test_portal_interop<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let values = std::fs::read_to_string(TEST_DATA_FILE_PATH)
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let content: Vec<(BeaconContentKey, BeaconContentValue)> = values.as_sequence().unwrap().iter().map(|value| {
            let content_key: BeaconContentKey =
                serde_yaml::from_value(value.get("content_key").unwrap().clone()).unwrap();
            let content_value: BeaconContentValue =
                serde_yaml::from_value(value.get("content_value").unwrap().clone()).unwrap();
            (content_key, content_value)
        }).collect();

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for (client_a, client_b) in clients.iter().cartesian_product(clients.iter()) {
            for ProcessedContent { content_type, identifier, test_data } in process_content(content.clone()) {
                test.run(
                    NClientTestSpec {
                        name: format!("OFFER {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_offer,
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                        test_data: Some(test_data.clone()),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("RecursiveFindContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_recursive_find_content,
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                        test_data: Some(test_data.clone()),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("FindContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_find_content,
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                        test_data: Some(test_data),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            // Test portal beacon ping
            test.run(NClientTestSpec {
                    name: format!("PING {} --> {}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_ping,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test find content non-present
            test.run(NClientTestSpec {
                    name: format!("FIND_CONTENT non present {} --> {}", client_a.name, client_b.name),
                    description: "find content: calls find content that doesn't exist".to_string(),
                    always_run: false,
                    run: test_find_content_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test find nodes distance zero
            test.run(NClientTestSpec {
                    name: format!("FIND_NODES Distance 0 {} --> {}", client_a.name, client_b.name),
                    description: "find nodes: distance zero expect called nodes enr".to_string(),
                    always_run: false,
                    run: test_find_nodes_zero_distance,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test gossiping a collection of blocks to node B (B will gossip back to A)
            test.run(
                NClientTestSpec {
                    name: format!("GOSSIP blocks from A:{} --> B:{}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_gossip_two_nodes,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: Some(content.clone().into_iter().map(content_pair_to_string_pair).collect()),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    // test that a node will not return content via FINDCONTENT.
    async fn test_find_content_non_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let header_with_proof_key: BeaconContentKey = serde_json::from_value(json!(BOOTSTRAP_KEY)).unwrap();

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
    async fn test_offer<'a>(clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
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
            let optional_key: BeaconContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: BeaconContentValue =
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
        let (target_key, target_value) = test_data.first().expect("Target content is required for this test");
        let target_key: BeaconContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: BeaconContentValue =
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
                if possible_content != target_value {
                    panic!("Error receiving content: Expected content: {target_value:?}, Received content: {possible_content:?}");
                }
            }
            Err(err) => {
                panic!("Unable to get received content: {err:?}");
            }
        }
    }
}

dyn_async! {
    async fn test_ping<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
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

        match BeaconNetworkApiClient::get_enr(&client_b.rpc, stored_enr.node_id()).await {
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
    async fn test_find_nodes_zero_distance<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
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

                match response.first() {
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
    async fn test_recursive_find_content<'a>(clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
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
            let optional_key: BeaconContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: BeaconContentValue =
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

        let (target_key, target_value) = test_data.first().expect("Target content is required for this test");
        let target_key: BeaconContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: BeaconContentValue =
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

        match BeaconNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        match client_a.rpc.recursive_find_content(target_key.clone()).await {
            Ok(result) => {
                match result {
                    ContentInfo::Content{ content, utp_transfer } => {
                        if content != target_value {
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
    async fn test_find_content<'a> (clients: Vec<Client>, test_data: Option<Vec<(String, String)>>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data {
            Some(test_data) => test_data,
            None => panic!("Expected test data none was provided"),
        };
        if let Some((optional_key, optional_value)) = test_data.get(1) {
            let optional_key: BeaconContentKey =
                serde_json::from_value(json!(optional_key)).unwrap();
            let optional_value: BeaconContentValue =
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

        let (target_key, target_value) = test_data.first().expect("Target content is required for this test");
        let target_key: BeaconContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_value: BeaconContentValue =
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
                    ContentInfo::Content{ content, utp_transfer } => {
                        if content != target_value {
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
        match BeaconNetworkApiClient::add_enr(&client_a.rpc, client_b_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // With default node settings nodes should be storing all content
        for (content_key, content_value) in test_data.clone() {
            let content_key: BeaconContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_value: BeaconContentValue =
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

            tokio::time::sleep(Duration::from_secs(1)).await;
        }

        // wait test_data.len() seconds for data to propagate, giving more time if more items are propagating
        tokio::time::sleep(Duration::from_secs(test_data.len() as u64)).await;

        // process raw test data to generate content details for error output
        let mut result = vec![];
        for (content_key, content_value) in test_data.into_iter() {
            let content_key: BeaconContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_value: BeaconContentValue =
                serde_json::from_value(json!(content_value)).unwrap();

            let content_details = {
                    let content_type = match &content_key {
                        BeaconContentKey::LightClientBootstrap(_) => "bootstrap".to_string(),
                        BeaconContentKey::LightClientUpdatesByRange(_) => "updates by range".to_string(),
                        BeaconContentKey::LightClientFinalityUpdate(_) => "finality update".to_string(),
                        BeaconContentKey::LightClientOptimisticUpdate(_) => "optimistic update".to_string(),
                    };
                    format!(
                        "{:?} {}",
                        content_key,
                        content_type
                    )
            };

            match client_b.rpc.local_content(content_key.clone()).await {
                Ok(expected_value) => {
                    if expected_value != content_value {
                        result.push(format!("Error content received for block {content_details} was different then expected"));
                    }
                }
                Err(err) => {
                    result.push(format!("Error content for block {err} was absent"));
                }
            }
        }

        if !result.is_empty() {
            panic!("Client B: {:?}", result);
        }
    }
}
