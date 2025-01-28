use std::str::FromStr;

use alloy_primitives::Bytes;
use ethportal_api::types::portal::{FindContentInfo, GetContentInfo, PutContentInfo};
use ethportal_api::types::portal_wire::MAX_PORTAL_CONTENT_PAYLOAD_SIZE;
use ethportal_api::{
    ContentValue, Discv5ApiClient, HistoryContentKey, HistoryContentValue, HistoryNetworkApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use serde_json::json;
use serde_yaml::Value;
use tokio::time::Duration;

use crate::suites::history::constants::{TEST_DATA_FILE_PATH, TRIN_BRIDGE_CLIENT_TYPE};
use crate::suites::utils::get_flair;

// Header with proof for block number 14764013
const HEADER_WITH_PROOF_KEY: &str =
    "0x00720704f3aa11c53cf344ea069db95cecb81ad7453c8f276b2a1062979611f09c";

type TestData = Vec<(HistoryContentKey, HistoryContentValue)>;

/// Processed content data for history tests
struct ProcessedContent {
    content_type: String,
    block_number: u64,
    test_data: TestData,
}

fn process_content(
    content: Vec<(HistoryContentKey, HistoryContentValue)>,
) -> Vec<ProcessedContent> {
    let mut last_header = content.first().unwrap().clone();

    let mut result: Vec<ProcessedContent> = vec![];
    for history_content in content.into_iter() {
        if let HistoryContentKey::BlockHeaderByHash(_) = &history_content.0 {
            last_header = history_content.clone();
        }
        let (content_type, block_number, test_data) =
            if let HistoryContentValue::BlockHeaderWithProof(header_with_proof) = &last_header.1 {
                match &history_content.0 {
                    HistoryContentKey::BlockHeaderByHash(_) => (
                        "Block Header by Hash".to_string(),
                        header_with_proof.header.number,
                        vec![last_header.clone()],
                    ),
                    HistoryContentKey::BlockHeaderByNumber(_) => (
                        "Block Header by Number".to_string(),
                        header_with_proof.header.number,
                        vec![history_content],
                    ),
                    HistoryContentKey::BlockBody(_) => (
                        "Block Body".to_string(),
                        header_with_proof.header.number,
                        vec![history_content, last_header.clone()],
                    ),
                    HistoryContentKey::BlockReceipts(_) => (
                        "Block Receipt".to_string(),
                        header_with_proof.header.number,
                        vec![history_content, last_header.clone()],
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

pub fn get_test_message(block_number: u64) -> String {
    if block_number == u64::MAX {
        " ".to_string()
    } else {
        format!(" block number {}{}", block_number, get_flair(block_number))
    }
}

dyn_async! {
   pub async fn test_portal_history_interop<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let values = std::fs::read_to_string(TEST_DATA_FILE_PATH)
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let content: Vec<(HistoryContentKey, HistoryContentValue)> = values.as_sequence().unwrap().iter().map(|value| {
            let content_key: HistoryContentKey =
                serde_yaml::from_value(value["content_key"].clone()).unwrap();
            let raw_content_value = Bytes::from_str(value["content_value"].as_str().unwrap()).unwrap();
            let content_value = HistoryContentValue::decode(&content_key, raw_content_value.as_ref()).expect("unable to decode content value");
            (content_key, content_value)
        }).collect();

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for (client_a, client_b) in clients.iter().cartesian_product(clients.iter()) {
            for ProcessedContent { content_type, block_number, test_data } in process_content(content.clone()) {
                test.run(
                    NClientTestSpec {
                        name: format!("OFFER {}:{} {} --> {}", content_type, get_test_message(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_offer,
                        environments: None,
                        test_data: test_data.clone(),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("GetContent {}:{} {} --> {}", content_type, get_test_message(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_get_content,
                        environments: None,
                        test_data: test_data.clone(),
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;

                test.run(
                    NClientTestSpec {
                        name: format!("FindContent {}:{} {} --> {}", content_type, get_test_message(block_number), client_a.name, client_b.name),
                        description: "".to_string(),
                        always_run: false,
                        run: test_find_content,
                        environments: None,
                        test_data,
                        clients: vec![client_a.clone(), client_b.clone()],
                    }
                ).await;
            }

            // Test portal history ping
            test.run(NClientTestSpec {
                    name: format!("PING {} --> {}", client_a.name, client_b.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_ping,
                    environments: None,
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
                    environments: None,
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
                    environments: None,
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
                    environments: None,
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
        let header_with_proof_key: HistoryContentKey = serde_json::from_value(json!(HEADER_WITH_PROOF_KEY)).unwrap();

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        let result = client_a.rpc.find_content(target_enr, header_with_proof_key.clone()).await;

        match result {
            Ok(FindContentInfo::Enrs { enrs }) => if !enrs.is_empty() {
                panic!("Error: Unexpected FINDCONTENT response: expected ContentInfo::Enrs length 0 got {}", enrs.len());
            }
            Ok(FindContentInfo::Content { .. }) => panic!("Error: Unexpected FINDCONTENT response: wasn't supposed to return back content"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_offer<'a>(clients: Vec<Client>, test_data: TestData) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        if let Some((optional_key, optional_value)) = test_data.get(1).cloned() {
            match client_b.rpc.store(optional_key, optional_value.encode()).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for get content");
                },
                Err(err) => panic!("Error storing optional content for get content: {err:?}"),
            }
        }
        let (target_key, target_value) = test_data.first().cloned().expect("Target content is required for this test");

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        let _ = client_a.rpc.offer(target_enr, vec![(target_key.clone(), target_value.encode())]).await;

        tokio::time::sleep(Duration::from_secs(8)).await;

        match client_b.rpc.local_content(target_key).await {
            Ok(possible_content) => {
                if possible_content != target_value.encode() {
                    panic!("Error receiving content: Expected content: {target_value:?}, Received content: {possible_content:?}");
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
                    Some(response_enr) => if *response_enr != target_enr {
                        panic!("Response from FindNodes didn't return expected Enr");
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
        if let Some((optional_key, optional_value)) = test_data.get(1).cloned() {
            match client_b.rpc.store(optional_key, optional_value.encode()).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for get content");
                },
                Err(err) => panic!("Error storing optional content for get content: {err:?}"),
            }
        }

        let (target_key, target_value) = test_data.first().cloned().expect("Target content is required for this test");
        match client_b.rpc.store(target_key.clone(), target_value.encode()).await {
            Ok(result) => if !result {
                panic!("Error storing target content for get content");
            },
            Err(err) => panic!("Error storing target content: {err:?}"),
        }

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        match HistoryNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        match client_a.rpc.get_content(target_key.clone()).await {
            Ok(GetContentInfo { content, utp_transfer }) => {
                if content != target_value.encode() {
                    panic!("Error: Unexpected GETCONTENT response: didn't return expected target content");
                }

                if target_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
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
        if let Some((optional_key, optional_value)) = test_data.get(1).cloned() {
            match client_b.rpc.store(optional_key, optional_value.encode()).await {
                Ok(result) => if !result {
                    panic!("Unable to store optional content for find content");
                },
                Err(err) => panic!("Error storing optional content for find content: {err:?}"),
            }
        }

        let (target_key, target_value) = test_data.first().cloned().expect("Target content is required for this test");
        match client_b.rpc.store(target_key.clone(), target_value.encode()).await {
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
                if content != target_value.encode() {
                    panic!("Error: Unexpected FINDCONTENT response: didn't return expected block body");
                }

                if target_value.encode().len() < MAX_PORTAL_CONTENT_PAYLOAD_SIZE {
                    if utp_transfer {
                        panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be false");
                    }
                } else if !utp_transfer {
                    panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be true");
                }
            }
            Ok(FindContentInfo::Enrs { .. }) => panic!("Error: Unexpected FINDCONTENT response: wasn't supposed to return back enrs"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_put_content_two_nodes<'a> (clients: Vec<Client>, test_data: TestData) {
        let (client_a, client_b) = match clients.iter().collect_tuple() {
            Some((client_a, client_b)) => (client_a, client_b),
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        // connect clients
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
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
            match client_a.rpc.put_content(content_key.clone(), content_value.encode()).await {
                Ok(PutContentInfo { peer_count, .. }) => {
                   if peer_count != 1 {
                        panic!("We expected to gossip to 1 node instead we gossiped to: {peer_count}");
                    }
                }
                Err(err) => panic!("Unable to get received content: {err:?}"),
            }

            if let HistoryContentKey::BlockHeaderByHash(_) = content_key {
                tokio::time::sleep(Duration::from_secs(1)).await;
            }
        }

        // wait test_data.len() seconds for data to propagate, giving more time if more items are propagating
        tokio::time::sleep(Duration::from_secs(test_data.len() as u64)).await;

        // process raw test data to generate content details for error output
        let (first_header_key, first_header_value) = test_data.first().cloned().unwrap();
        let mut last_header_seen: (HistoryContentKey, HistoryContentValue) = (first_header_key, first_header_value);
        let mut result = vec![];
        for (content_key, content_value) in test_data.into_iter() {
            if let HistoryContentKey::BlockHeaderByHash(_) = &content_key {
                last_header_seen = (content_key.clone(), content_value.clone());
            }
            let content_details =
                if let HistoryContentValue::BlockHeaderWithProof(header_with_proof) = &last_header_seen.1 {
                    let content_type = match &content_key {
                        HistoryContentKey::BlockHeaderByHash(_) => "header by hash".to_string(),
                        HistoryContentKey::BlockHeaderByNumber(_) => "header by number".to_string(),
                        HistoryContentKey::BlockBody(_) => "body".to_string(),
                        HistoryContentKey::BlockReceipts(_) => "receipt".to_string(),
                    };
                    format!(
                        "{} {}",
                        get_test_message(header_with_proof.header.number),
                        content_type
                    )
                } else {
                    unreachable!("History test data is formatted incorrectly. Header wasn't in front of data. Please refer to test data file for more information")
                };

            match client_b.rpc.local_content(content_key.clone()).await {
                Ok(expected_value) => if expected_value != content_value.encode() {
                    result.push(format!("Error content received for block {content_details} was different then expected"));
                }
                Err(err) => result.push(format!("Error content for block {err} was absent")),
            }
        }

        if !result.is_empty() {
            panic!("Client B: {:?}", result);
        }
    }
}
