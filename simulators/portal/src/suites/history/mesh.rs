use alloy_primitives::Bytes;
use ethportal_api::types::distance::{Metric, XorMetric};
use ethportal_api::types::portal::FindContentInfo;
use ethportal_api::{
    BeaconNetworkApiClient, ContentValue, Discv5ApiClient, HistoryContentKey,
    HistoryNetworkApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use std::collections::HashMap;

use crate::suites::environment::PortalNetwork;
use crate::suites::history::constants::{
    HEADER_WITH_PROOF_KEY, HEADER_WITH_PROOF_VALUE, TRIN_BRIDGE_CLIENT_TYPE,
};

use super::constants::LATEST_HISTORICAL_SUMMARIES;

// private key hive environment variable
const PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";

dyn_async! {
   pub async fn test_portal_history_mesh<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let private_key_1 = "fc34e57cc83ed45aae140152fd84e2c21d1f4d46e19452e13acc7ee90daa5bac".to_string();
        let private_key_2 = "e5add57dc4c9ef382509e61ce106ec86f60eb73bbfe326b00f54bf8e1819ba11".to_string();

        let selected_networks = PortalNetwork::as_environment_flag([PortalNetwork::Beacon, PortalNetwork::History]);

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for ((client_a, client_b), client_c) in clients.iter().cartesian_product(clients.iter()).cartesian_product(clients.iter()) {
            test.run(
                NClientTestSpec {
                    name: format!("FIND_CONTENT content stored 2 nodes away stored in client C (Client B closer to content then C). A:{} --> B:{} --> C:{}", client_a.name, client_b.name, client_c.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_find_content_two_jumps,
                    environments: Some(vec![Some(HashMap::from([selected_networks.clone()])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_2.clone()), selected_networks.clone()])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_1.clone()), selected_networks.clone()]))]),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone(), client_c.clone()],
                }
            ).await;

            // Remove this after the clients are stable across two jumps test
            test.run(
                NClientTestSpec {
                    name: format!("FIND_CONTENT content stored 2 nodes away stored in client C (Client C closer to content then B). A:{} --> B:{} --> C:{}", client_a.name, client_b.name, client_c.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_find_content_two_jumps,
                    environments: Some(vec![Some(HashMap::from([selected_networks.clone()])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_1.clone()), selected_networks.clone()])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_2.clone()), selected_networks.clone()]))]),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone(), client_c.clone()],
                }
            ).await;

            // Test find nodes distance of client a
            test.run(NClientTestSpec {
                    name: format!("FIND_NODES distance of client C {} --> {} --> {}", client_a.name, client_b.name, client_c.name),
                    description: "find nodes: distance of client A expect seeded enr returned".to_string(),
                    always_run: false,
                    run: test_find_nodes_distance_of_client_c,
                    environments: Some(vec![Some(HashMap::from([selected_networks.clone()])), Some(HashMap::from([selected_networks.clone()])), Some(HashMap::from([selected_networks.clone()]))]),
                    test_data: (),
                    clients: vec![client_a.clone(), client_b.clone(), client_c.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    async fn test_find_content_two_jumps<'a> (clients: Vec<Client>, _: ()) {
        let (client_a, client_b, client_c) = initialize_clients(clients).await;

        let header_with_proof_key: &HistoryContentKey = &HEADER_WITH_PROOF_KEY;
        let raw_header_with_proof_value: &Bytes = &HEADER_WITH_PROOF_VALUE;

        // get enr for b and c to seed for the jumps
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        let client_c_enr = match client_c.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // seed client_c_enr into routing table of client_b
        match HistoryNetworkApiClient::add_enr(&client_b.rpc, client_c_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // send a ping from client B to C to connect the clients
        if let Err(err) = HistoryNetworkApiClient::ping(&client_b.rpc, client_c_enr.clone(), None, None).await {
            panic!("Unable to receive pong info: {err:?}");
        }

        // seed the data into client_c
        match HistoryNetworkApiClient::store(&client_c.rpc, header_with_proof_key.clone(), raw_header_with_proof_value.clone()).await {
            Ok(result) => if !result {
                panic!("Unable to store header with proof for find content immediate return test");
            },
            Err(err) => panic!("Error storing header with proof for find content immediate return test: {err:?}"),
        }

        let enrs = match  HistoryNetworkApiClient::find_content(&client_a.rpc, client_b_enr.clone(), header_with_proof_key.clone()).await {
            Ok(FindContentInfo::Enrs{ enrs }) => enrs,
            Ok(FindContentInfo::Content{ .. }) => panic!("Error: Unexpected FINDCONTENT response: expected Enrs instead got Content"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        };

        if enrs.len() != 1 {
            panic!("Known node is closer to content, Enrs returned should be 0 instead got: length {}", enrs.len());
        }

        match HistoryNetworkApiClient::find_content(&client_a.rpc, enrs[0].clone(), header_with_proof_key.clone()).await {
            Ok(FindContentInfo::Content { content, utp_transfer }) => {
                if content != raw_header_with_proof_value.clone() {
                    panic!("Error: Unexpected FINDCONTENT response: didn't return expected header with proof value");
                }

                if utp_transfer {
                    panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be false");
                }
            },
            Ok(FindContentInfo::Enrs { .. }) => panic!("Error: Unexpected FINDCONTENT response: expected Content instead got Enrs"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}

dyn_async! {
    async fn test_find_nodes_distance_of_client_c<'a>(clients: Vec<Client>, _: ()) {
        let (client_a, client_b, client_c) = initialize_clients(clients).await;

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // We are adding client C to our list so we then can assume only one client per bucket
        let client_c_enr = match client_c.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // seed enr into routing table
        match HistoryNetworkApiClient::add_enr(&client_b.rpc, client_c_enr.clone()).await {
            Ok(response) => if !response {
                panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        if let Some(distance) = XorMetric::distance(&target_enr.node_id().raw(), &client_c_enr.node_id().raw()).log2() {
            match HistoryNetworkApiClient::find_nodes(&client_a.rpc, target_enr.clone(), vec![distance as u16]).await {
                Ok(response) => {
                    if response.is_empty() {
                        panic!("FindNodes expected to have received a non-empty response");
                    }

                    if !response.contains(&client_c_enr) {
                        panic!("FindNodes {distance} distance expected to contained seeded Enr");
                    }
                }
                Err(err) => panic!("{}", &err.to_string()),
            }
        } else {
            panic!("Distance calculation failed");
        }
    }
}

/// Initialize clients for the test
///
/// It will insert HistoricalSummariesWithProof content into the clients
///
/// Panics if the clients are not of length 3, or if HistoricalSummaries can't be seeded
async fn initialize_clients(clients: Vec<Client>) -> (Client, Client, Client) {
    let (content_key, content_value) = LATEST_HISTORICAL_SUMMARIES.as_ref().clone();

    for client in clients.iter() {
        match BeaconNetworkApiClient::store(
            &client.rpc,
            content_key.clone(),
            content_value.encode(),
        )
        .await
        {
            Ok(result) => {
                if !result {
                    panic!(
                        "Unable to HistoricalSummaries for client_type {}",
                        client.kind
                    );
                }
            }
            Err(err) => panic!(
                "Error storing HistoricalSummaries for client_type {}: {err:?}",
                client.kind
            ),
        }
    }

    match clients.into_iter().collect_tuple() {
        Some((client_a, client_b, client_c)) => (client_a, client_b, client_c),
        None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
    }
}
