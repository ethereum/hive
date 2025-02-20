use std::collections::HashMap;

use crate::suites::beacon::constants::{
    get_test_data, BOOTSTRAP_CONTENT_KEY, TRIN_BRIDGE_CLIENT_TYPE,
};
use crate::suites::environment::PortalNetwork;
use ethportal_api::types::portal::{FindContentInfo, GetContentInfo};
use ethportal_api::types::portal_wire::MAX_PORTAL_CONTENT_PAYLOAD_SIZE;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{
    BeaconContentKey, BeaconContentValue, BeaconNetworkApiClient, ContentValue, Discv5ApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;

type TestData = (BeaconContentKey, BeaconContentValue);

/// Processed content data for beacon tests
struct ProcessedContent {
    content_type: String,
    identifier: String,
    test_data: TestData,
}

fn process_content(content: Vec<(BeaconContentKey, BeaconContentValue)>) -> Vec<ProcessedContent> {
    let mut result: Vec<ProcessedContent> = vec![];
    for beacon_content in content.into_iter() {
        let (content_type, identifier, test_data) = match &beacon_content.0 {
            BeaconContentKey::LightClientBootstrap(bootstrap) => (
                "Bootstrap".to_string(),
                hex_encode(bootstrap.block_hash),
                beacon_content,
            ),
            BeaconContentKey::LightClientUpdatesByRange(updates_by_range) => (
                "Updates by Range".to_string(),
                format!(
                    "start period: {} count: {}",
                    updates_by_range.start_period, updates_by_range.count
                ),
                beacon_content,
            ),
            BeaconContentKey::LightClientFinalityUpdate(finality_update) => (
                "Finality Update".to_string(),
                format!("finalized slot: {}", finality_update.finalized_slot),
                beacon_content,
            ),
            BeaconContentKey::LightClientOptimisticUpdate(optimistic_update) => (
                "Optimistic Update".to_string(),
                format!("optimistic slot: {}", optimistic_update.signature_slot),
                beacon_content,
            ),
            BeaconContentKey::HistoricalSummariesWithProof(historical_summaries) => (
                "Historical Summaries".to_string(),
                format!("historical summaries epoch: {}", historical_summaries.epoch),
                beacon_content,
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
   pub async fn test_portal_beacon_interop<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let environment = Some(HashMap::from([PortalNetwork::as_environment_flag([PortalNetwork::Beacon])]));
        let environments = Some(vec![environment.clone(), environment]);

        let content = get_test_data().expect("cannot parse test asset");

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for (client_a, client_b) in clients.iter().cartesian_product(clients.iter()) {
            for ProcessedContent { content_type, identifier, test_data } in process_content(content.clone()) {
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

            // Test portal beacon ping
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
        let bootstrap_key = BOOTSTRAP_CONTENT_KEY.clone();

        let target_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        match client_a.rpc.find_content(target_enr, bootstrap_key.clone()).await {
            Ok(FindContentInfo::Enrs { enrs }) => if !enrs.is_empty() {
                panic!("Error: Unexpected FINDCONTENT response: expected ContentInfo::Enrs length 0 got {}", enrs.len());
            },
            Ok(FindContentInfo::Content { .. }) => panic!("Error: Unexpected FINDCONTENT response: wasn't supposed to return back content"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
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
        let (target_key, target_value) = test_data;
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

        match BeaconNetworkApiClient::add_enr(&client_a.rpc, target_enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        match client_a.rpc.get_content(target_key.clone()).await {
            Ok(GetContentInfo{ content, utp_transfer }) => {
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
        let (target_key, target_value) = test_data;
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
            Ok(FindContentInfo::Content{ content, utp_transfer }) => {
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
            },
            Ok(FindContentInfo::Enrs { .. }) =>  panic!("Error: Unexpected FINDCONTENT received Enrs instead of Content"),
            Err(err) => panic!("Error: Unable to get response from FINDCONTENT request: {err:?}"),
        }
    }
}
