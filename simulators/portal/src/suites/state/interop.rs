use std::collections::HashMap;
use std::str::FromStr;

use crate::suites::environment::PortalNetwork;
use crate::suites::state::constants::{CONTENT_KEY, TEST_DATA_FILE_PATH, TRIN_BRIDGE_CLIENT_TYPE};
use alloy_primitives::Bytes;
use alloy_rlp::Decodable;
use ethportal_api::jsonrpsee::http_client::HttpClient;
use ethportal_api::types::execution::header_with_proof::{
    BlockHeaderProof, HeaderWithProof, SszNone,
};
use ethportal_api::types::portal::{FindContentInfo, GetContentInfo, PutContentInfo};
use ethportal_api::types::portal_wire::MAX_PORTAL_CONTENT_PAYLOAD_SIZE;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{
    ContentValue, Discv5ApiClient, Header, HistoryContentKey, HistoryContentValue, StateContentKey,
    StateContentValue, StateNetworkApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use serde_json::json;
use serde_yaml::Value;
use tokio::time::Duration;

#[derive(Clone, Debug)]
struct TestData {
    pub header: Header,
    pub key: StateContentKey,
    pub offer_value: StateContentValue,
    pub lookup_value: StateContentValue,
}

async fn store_header(header: Header, client: &HttpClient) -> bool {
    let content_key = HistoryContentKey::new_block_header_by_hash(header.hash());
    let content_value = HistoryContentValue::BlockHeaderWithProof(HeaderWithProof {
        header,
        proof: BlockHeaderProof::None(SszNone::default()),
    });
    match ethportal_api::HistoryNetworkApiClient::store(client, content_key, content_value.encode())
        .await
    {
        Ok(stored) => stored,
        Err(err) => panic!("{}", &err.to_string()),
    }
}

/// Processed content data for state tests
struct ProcessedContent {
    content_type: String,
    identifier: String,
    test_data: TestData,
}

fn process_content(content: Vec<TestData>) -> Vec<ProcessedContent> {
    let mut result: Vec<ProcessedContent> = vec![];
    for test_data in content {
        let (content_type, identifier) = match &test_data.key {
            StateContentKey::AccountTrieNode(account_trie_node) => (
                "Account Trie Node".to_string(),
                format!(
                    "path: {} node hash: {}",
                    hex_encode(account_trie_node.path.nibbles()),
                    hex_encode(account_trie_node.node_hash.as_slice())
                ),
            ),
            StateContentKey::ContractStorageTrieNode(contract_storage_trie_node) => (
                "Contract Storage Trie Node".to_string(),
                format!(
                    "address: {} path: {} node hash: {}",
                    hex_encode(contract_storage_trie_node.address_hash.as_slice()),
                    hex_encode(contract_storage_trie_node.path.nibbles()),
                    hex_encode(contract_storage_trie_node.node_hash.as_slice())
                ),
            ),
            StateContentKey::ContractBytecode(contract_bytecode) => (
                "Contract Bytecode".to_string(),
                format!(
                    "address: {} code hash: {}",
                    hex_encode(contract_bytecode.address_hash.as_slice()),
                    hex_encode(contract_bytecode.code_hash.as_slice())
                ),
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
   pub async fn test_portal_state_interop<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let environment = Some(HashMap::from([PortalNetwork::as_environment_flag([PortalNetwork::State, PortalNetwork::History])]));
        let environments = Some(vec![environment.clone(), environment]);

        let values = std::fs::read_to_string(TEST_DATA_FILE_PATH)
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let content: Vec<TestData> = values.as_sequence().unwrap().iter().map(|value| {
            let header_bytes: Bytes = serde_yaml::from_value(value["block_header"].clone()).unwrap();
            let header = Header::decode(&mut header_bytes.as_ref()).unwrap();
            let key: StateContentKey =
                serde_yaml::from_value(value["content_key"].clone()).unwrap();
            let raw_offer_value = Bytes::from_str(value["content_value_offer"].as_str().unwrap()).unwrap();
            let offer_value = StateContentValue::decode(&key, raw_offer_value.as_ref()).expect("unable to decode content value");
            let raw_lookup_value = Bytes::from_str(value["content_value_retrieval"].as_str().unwrap()).unwrap();
            let lookup_value = StateContentValue::decode(&key, raw_lookup_value.as_ref()).expect("unable to decode content value");

            TestData { header, key, offer_value, lookup_value }
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
                        environments: environments.clone(),
                        test_data: test_data.clone(),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("GetContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_get_content,
                        environments: environments.clone(),
                        test_data: test_data.clone(),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("FindContent {}: {} {} --> {}", content_type, identifier, client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_find_content,
                        environments: environments.clone(),
                        test_data,
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
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test find content non-present
            test.run(NClientTestSpec {
                    name: format!("FIND_CONTENT non present {} --> {}", client_a.name, client_b.name),
                    description: "find content: calls find content that doesn't exist".to_string(),
                    always_run: false,
                    run: test_find_content_non_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test find nodes distance zero
            test.run(NClientTestSpec {
                    name: format!("FIND_NODES Distance 0 {} --> {}", client_a.name, client_b.name),
                    description: "find nodes: distance zero expect called nodes enr".to_string(),
                    always_run: false,
                    run: test_find_nodes_zero_distance,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;

            // Test gossiping a collection of blocks to node B (B will gossip back to A)
            test.run(
                NClientTestSpec {
                    name: format!("PUT CONTENT blocks from A:{} --> B:{}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_put_content_two_nodes,
                    environments: environments.clone(),
                    test_data: content.clone(),
                    clients: vec![client_a.clone(), client_b.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    // test that a node will not return content via FINDCONTENT.
    async fn test_find_content_non_present<'a>(clients: Vec<Client>, _: ()) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let header_with_proof_key: StateContentKey = serde_json::from_value(json!(CONTENT_KEY)).unwrap();

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        let result = client_a.rpc.find_content(target_enr, header_with_proof_key.clone()).await;

        match result {
            Ok(FindContentInfo::Enrs { enrs }) => if !enrs.is_empty() {
                panic!("Error: Unexpected FINDCONTENT response: expected ContentInfo::Enrs length 0 got {}", enrs.len());
            },
            Ok(FindContentInfo::Content { .. }) => panic!("Error: Unexpected FINDCONTENT response: expected Enrs got Content"),
            Err(err) =>  panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_offer<'a>(clients: Vec<Client>, test_data: TestData) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };

        let TestData {
            header,
            key: target_key,
            offer_value: target_offer_value,
            lookup_value: target_lookup_value
        } = test_data;

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        store_header(header, &client_b.rpc).await;

        let _ = client_a.rpc.offer(target_enr, vec![(target_key.clone(), target_offer_value.encode())]).await;

        tokio::time::sleep(Duration::from_secs(8)).await;

        match client_b.rpc.local_content(target_key).await {
            Ok(possible_content) => {
                if possible_content != target_lookup_value.encode() {
                    panic!("Error receiving content: Expected content: {target_lookup_value:?}, Received content: {possible_content:?}");
                }
            }
            Err(err) => panic!("Unable to get received content: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_ping<'a>(clients: Vec<Client>, _: ()) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        let pong = client_a.rpc.ping(target_enr).await;

        if let Err(err) = pong {
                panic!("Unable to receive pong info: {err:?}");
        }

        // Verify that client_b stored client_a its ENR through the base layer
        // handshake mechanism.
        let stored_enr = match client_a.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
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
    async fn test_find_nodes_zero_distance<'a>(clients: Vec<Client>, _: ()) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
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
    // test that a node will return a content via GETCONTENT template that it has stored locally
    async fn test_get_content<'a>(clients: Vec<Client>, test_data: TestData) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };

        let TestData {
            key: target_key,
            offer_value: target_offer_value,
            lookup_value: target_lookup_value,
            ..
        } = test_data;

        match client_b.rpc.store(target_key.clone(), target_offer_value.encode()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for get content");
            },
            Err(err) => panic!("Error storing target content: {err:?}"),
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        match StateNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        match client_a.rpc.get_content(target_key.clone()).await {
            Ok(GetContentInfo{ content, utp_transfer }) => {
                if content != target_lookup_value.encode() {
                    panic!("Error: Unexpected GETCONTENT response: didn't return expected target content");
                }

                if target_lookup_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
                    if utp_transfer {
                        panic!("Error: Unexpected GETCONTENT response: utp_transfer was supposed to be false");
                    }
                } else if !utp_transfer {
                    panic!("Error: Unexpected GETCONTENT response: utp_transfer was supposed to be true");
                }
            },
            Err(err) => panic!("Error: Unable to get response from GETCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    // test that a node will return a x content via FINDCONTENT that it has stored locally
    async fn test_find_content<'a> (clients: Vec<Client>, test_data: TestData) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };

        let TestData {
            key: target_key,
            offer_value: target_offer_value,
            lookup_value: target_lookup_value,
            ..
        } = test_data;

        match client_b.rpc.store(target_key.clone(), target_offer_value.encode()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for find content");
            },
            Err(err) => panic!("Error storing target content: {err:?}"),
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        match client_a.rpc.find_content(target_enr, target_key.clone()).await {
            Ok(FindContentInfo::Content { content, utp_transfer }) => {
                if content != target_lookup_value.encode() {
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
            Ok(FindContentInfo::Enrs { .. }) => panic!("Error: Unexpected FINDCONTENT response: expected Content got Enrs"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_put_content_two_nodes<'a> (clients: Vec<Client>, test_data: Vec<TestData>) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        // connect clients
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };
        match StateNetworkApiClient::add_enr(&client_a.rpc, client_b_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // With default node settings nodes should be storing all content
        for TestData { header, key: content_key, offer_value: content_offer_value, .. } in test_data.clone() {
            store_header(header, &client_b.rpc).await;

            match client_a.rpc.put_content(content_key.clone(), content_offer_value.encode()).await {
                Ok(PutContentInfo { peer_count, .. }) => {
                   if peer_count != 1 {
                        panic!("We expected to gossip to 1 node instead we gossiped to: {peer_count}");
                    }
                }
                Err(err) => panic!("Unable to get received content: {err:?}"),
            }

            tokio::time::sleep(Duration::from_secs(1)).await;
        }

        // wait test_data.len() seconds for data to propagate, giving more time if more items are propagating
        tokio::time::sleep(Duration::from_secs(test_data.len() as u64)).await;

        // process raw test data to generate content details for error output
        let mut result = vec![];
        for TestData { key: content_key, lookup_value: content_lookup_value, .. } in test_data {
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
                    if expected_value != content_lookup_value.encode() {
                        result.push(format!("Error content received for block {content_details} was different then expected"));
                    }
                }
                Err(err) => result.push(format!("Error content for block {err} was absent")),
            }
        }

        if !result.is_empty() {
            panic!("Client B: {:?}", result);
        }
    }
}
