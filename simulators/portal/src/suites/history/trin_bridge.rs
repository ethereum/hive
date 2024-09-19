use super::constants::{
    BOOTNODES_ENVIRONMENT_VARIABLE, HIVE_CHECK_LIVE_PORT, TEST_DATA_FILE_PATH,
    TRIN_BRIDGE_CLIENT_TYPE,
};
use crate::suites::utils::get_flair;
use alloy_primitives::Bytes;
use ethportal_api::ContentValue;
use ethportal_api::HistoryContentKey;
use ethportal_api::HistoryContentValue;
use ethportal_api::{Discv5ApiClient, HistoryNetworkApiClient};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use serde_yaml::Value;
use std::collections::HashMap;
use std::str::FromStr;
use tokio::time::Duration;

fn process_content(content: Vec<(HistoryContentKey, HistoryContentValue)>) -> Vec<String> {
    let mut last_header = content.first().unwrap().clone();

    let mut result: Vec<String> = vec![];
    for history_content in content.into_iter() {
        if let HistoryContentKey::BlockHeaderByHash(_) = &history_content.0 {
            last_header = history_content.clone();
        }
        let comment =
            if let HistoryContentValue::BlockHeaderWithProof(header_with_proof) = &last_header.1 {
                let content_type = match &history_content.0 {
                    HistoryContentKey::BlockHeaderByHash(_) => "header by hash".to_string(),
                    HistoryContentKey::BlockHeaderByNumber(_) => "header by number".to_string(),
                    HistoryContentKey::BlockBody(_) => "body".to_string(),
                    HistoryContentKey::BlockReceipts(_) => "receipt".to_string(),
                };
                format!(
                    "{}{} {}",
                    header_with_proof.header.number,
                    get_flair(header_with_proof.header.number),
                    content_type
                )
            } else {
                unreachable!("History test dated is formatted incorrectly")
            };
        result.push(comment)
    }
    result
}

dyn_async! {
   pub async fn test_portal_bridge<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        if !clients.iter().any(|client_definition| client_definition.name == *TRIN_BRIDGE_CLIENT_TYPE) {
            panic!("This simulator is required to be ran with client `trin-bridge`")
        }
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for client in &clients {
            test.run(
                NClientTestSpec {
                    name: format!("Bridge test. A:Trin Bridge --> B:{}", client.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_bridge,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    async fn test_bridge<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };

        let client_enr = match client.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };
        client.test.start_client(TRIN_BRIDGE_CLIENT_TYPE.to_string(), Some(HashMap::from([(BOOTNODES_ENVIRONMENT_VARIABLE.to_string(), client_enr.to_base64()), (HIVE_CHECK_LIVE_PORT.to_string(), 0.to_string())]))).await;

        // With default node settings nodes should be storing all content
        let values = std::fs::read_to_string(TEST_DATA_FILE_PATH)
            .expect("cannot find test asset");
        let values: Value = serde_yaml::from_str(&values).unwrap();
        let content_vec: Vec<(HistoryContentKey, HistoryContentValue)> = values.as_sequence().unwrap().iter().map(|value| {
            let content_key: HistoryContentKey =
                serde_yaml::from_value(value["content_key"].clone()).unwrap();
                let raw_content_value = Bytes::from_str(value["content_value"].as_str().unwrap()).unwrap();
                let content_value = HistoryContentValue::decode(&content_key, raw_content_value.as_ref()).expect("unable to decode content value");
                (content_key, content_value)
        }).collect();
        let comments = process_content(content_vec.clone());

        // wait content_vec.len() seconds for data to propagate, giving more time if more items are propagating
        tokio::time::sleep(Duration::from_secs(content_vec.len() as u64) * 2).await;

        let mut result = vec![];
        for (index, (content_key, content_value)) in content_vec.into_iter().enumerate() {
            match client.rpc.local_content(content_key.clone()).await {
                Ok(content) => {
                    if content != content_value.encode() {
                        result.push(format!("Error content received for block {} was different then expected: Provided: {content:?} Expected: {content_value:?}", comments[index]));
                    }
                }
                Err(err) => {
                    panic!("Unable to get received content: {err:?}");
                }
            }
        }

        if !result.is_empty() {
            panic!("Client: {:?}", result);
        }
    }
}
