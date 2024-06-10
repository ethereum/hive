use crate::suites::constants::{
    CONTENT_KEY, HIVE_PORTAL_NETWORKS_SELECTED, STATE_STRING, TEST_DATA_FILE_PATH,
    TRIN_BRIDGE_CLIENT_TYPE,
};
use ethportal_api::types::state::ContentInfo;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{
    ContentValue, Discv5ApiClient, OverlayContentKey, StateContentKey, StateContentValue,
    StateNetworkApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::types::ContentKeyOfferLookupValues;
use hivesim::types::TestData;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use serde_json::json;
use serde_yaml::Value;
use std::collections::HashMap;
use tokio::time::Duration;

// This is taken from Trin. It should be fairly standard
const MAX_PORTAL_CONTENT_PAYLOAD_SIZE: usize = 1165;

fn content_pair_to_string_pair(
    content_pair: (StateContentKey, StateContentValue, StateContentValue),
) -> ContentKeyOfferLookupValues {
    let (content_key, content_offer_value, content_lookup_value) = content_pair;
    ContentKeyOfferLookupValues {
        key: content_key.to_hex(),
        offer_value: hex_encode(content_offer_value.encode()),
        lookup_value: hex_encode(content_lookup_value.encode()),
    }
}

/// Processed content data for state tests
struct ProcessedContent {
    content_type: String,
    identifier: String,
    test_data: Vec<ContentKeyOfferLookupValues>,
}

fn process_content(
    content: Vec<(StateContentKey, StateContentValue, StateContentValue)>,
) -> Vec<ProcessedContent> {
    let mut result: Vec<ProcessedContent> = vec![];
    for state_content in content.into_iter() {
        let (content_type, identifier, test_data) = match &state_content.0 {
            StateContentKey::AccountTrieNode(account_trie_node) => (
                "Account Trie Node".to_string(),
                format!(
                    "path: {} node hash: {}",
                    hex_encode(account_trie_node.path.nibbles()),
                    hex_encode(account_trie_node.node_hash.as_bytes())
                ),
                vec![content_pair_to_string_pair(state_content)],
            ),
            StateContentKey::ContractStorageTrieNode(contract_storage_trie_node) => (
                "Contract Storage Trie Node".to_string(),
                format!(
                    "address: {} path: {} node hash: {}",
                    hex_encode(contract_storage_trie_node.address.as_bytes()),
                    hex_encode(contract_storage_trie_node.path.nibbles()),
                    hex_encode(contract_storage_trie_node.node_hash.as_bytes())
                ),
                vec![content_pair_to_string_pair(state_content)],
            ),
            StateContentKey::ContractBytecode(contract_bytecode) => (
                "Contract Bytecode".to_string(),
                format!(
                    "address: {} code hash: {}",
                    hex_encode(contract_bytecode.address.as_bytes()),
                    hex_encode(contract_bytecode.code_hash.as_bytes())
                ),
                vec![content_pair_to_string_pair(state_content)],
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
        let content: Vec<(StateContentKey, StateContentValue, StateContentValue)> = values.as_sequence().unwrap().iter().map(|value| {
            let content_key: StateContentKey =
                serde_yaml::from_value(value.get("content_key").unwrap().clone()).unwrap();
            let content_offer_value: StateContentValue =
                serde_yaml::from_value(value.get("content_value_offer").unwrap().clone()).unwrap();
            let content_lookup_value: StateContentValue =
                serde_yaml::from_value(value.get("content_value_retrieval").unwrap().clone()).unwrap();
            (content_key, content_offer_value, content_lookup_value)
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
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                        test_data: Some(TestData::StateContentList(test_data.clone())),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("RecursiveFindContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_recursive_find_content,
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                        test_data: Some(TestData::StateContentList(test_data.clone())),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("FindContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_find_content,
                        environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                        test_data: Some(TestData::StateContentList(test_data)),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            // Test portal state ping
            test.run(NClientTestSpec {
                    name: format!("PING {} --> {}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_ping,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
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
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
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
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
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
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())])), Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: Some(TestData::StateContentList(content.clone().into_iter().map(content_pair_to_string_pair).collect())),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    // test that a node will not return content via FINDCONTENT.
    async fn test_find_content_non_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let header_with_proof_key: StateContentKey = serde_json::from_value(json!(CONTENT_KEY)).unwrap();

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
    async fn test_offer<'a>(clients: Vec<Client>, test_data: Option<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data.map(|data| data.state_content_list()) {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };
        let ContentKeyOfferLookupValues { key: target_key, offer_value: target_offer_value, lookup_value: target_lookup_value } = test_data.first().expect("Target content is required for this test");
        let target_key: StateContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_offer_value: StateContentValue =
            serde_json::from_value(json!(target_offer_value)).unwrap();
        let target_lookup_value: StateContentValue =
            serde_json::from_value(json!(target_lookup_value)).unwrap();

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        let _ = client_a.rpc.offer(target_enr, target_key.clone(), Some(target_offer_value.clone())).await;

        tokio::time::sleep(Duration::from_secs(8)).await;

        match client_b.rpc.local_content(target_key).await {
            Ok(possible_content) => {
                if possible_content != target_lookup_value {
                    panic!("Error receiving content: Expected content: {target_lookup_value:?}, Received content: {possible_content:?}");
                }
            }
            Err(err) => {
                panic!("Unable to get received content: {err:?}");
            }
        }
    }
}

dyn_async! {
    async fn test_ping<'a>(clients: Vec<Client>, _: Option<TestData>) {
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

        match StateNetworkApiClient::get_enr(&client_b.rpc, stored_enr.node_id()).await {
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
    async fn test_find_nodes_zero_distance<'a>(clients: Vec<Client>, _: Option<TestData>) {
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
    async fn test_recursive_find_content<'a>(clients: Vec<Client>, test_data: Option<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data.map(|data| data.state_content_list()) {
            Some(test_data) => test_data,
            None => panic!("Expected test data non was provided"),
        };

        let ContentKeyOfferLookupValues { key: target_key, offer_value: target_offer_value, lookup_value: target_lookup_value } = test_data.first().expect("Target content is required for this test");
        let target_key: StateContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_offer_value: StateContentValue =
            serde_json::from_value(json!(target_offer_value)).unwrap();
        let target_lookup_value: StateContentValue =
            serde_json::from_value(json!(target_lookup_value)).unwrap();
        match client_b.rpc.store(target_key.clone(), target_offer_value.clone()).await {
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

        match StateNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
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
                        if content != target_lookup_value {
                            panic!("Error: Unexpected RECURSIVEFINDCONTENT response: didn't return expected target content");
                        }

                        if target_lookup_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
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
    async fn test_find_content<'a> (clients: Vec<Client>, test_data: Option<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data.map(|data| data.state_content_list()) {
            Some(test_data) => test_data,
            None => panic!("Expected test data none was provided"),
        };

        let ContentKeyOfferLookupValues { key: target_key, offer_value: target_offer_value, lookup_value: target_lookup_value } = test_data.first().expect("Target content is required for this test");
        let target_key: StateContentKey =
            serde_json::from_value(json!(target_key)).unwrap();
        let target_offer_value: StateContentValue =
            serde_json::from_value(json!(target_offer_value)).unwrap();
        let target_lookup_value: StateContentValue =
            serde_json::from_value(json!(target_lookup_value)).unwrap();
        match client_b.rpc.store(target_key.clone(), target_offer_value.clone()).await {
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
                        if content != target_lookup_value {
                            panic!("Error: Unexpected FINDCONTENT response: didn't return expected block body");
                        }

                        if target_lookup_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
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
    async fn test_gossip_two_nodes<'a> (clients: Vec<Client>, test_data: Option<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let test_data = match test_data.map(|data| data.state_content_list()) {
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
        match StateNetworkApiClient::add_enr(&client_a.rpc, client_b_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // With default node settings nodes should be storing all content
        for ContentKeyOfferLookupValues { key: content_key, offer_value: content_offer_value, lookup_value: _ } in test_data.clone().into_iter() {
            let content_key: StateContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_offer_value: StateContentValue =
                serde_json::from_value(json!(content_offer_value)).unwrap();

            match client_a.rpc.gossip(content_key.clone(), content_offer_value.clone()).await {
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
        for ContentKeyOfferLookupValues { key: content_key, offer_value: _, lookup_value: content_lookup_value } in test_data.into_iter() {
            let content_key: StateContentKey =
                serde_json::from_value(json!(content_key)).unwrap();
            let content_lookup_value: StateContentValue =
                serde_json::from_value(json!(content_lookup_value)).unwrap();

            let content_details = {
                    let content_type = match &content_key {
                        StateContentKey::AccountTrieNode(_) => "account trie node".to_string(),
                        StateContentKey::ContractStorageTrieNode(_) => "contract storage trie node".to_string(),
                        StateContentKey::ContractBytecode(_) => "contract bytecode".to_string(),
                    };
                    format!(
                        "{:?} {}",
                        content_key,
                        content_type
                    )
            };

            match client_b.rpc.local_content(content_key.clone()).await {
                Ok(expected_value) => {
                    if expected_value != content_lookup_value {
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
