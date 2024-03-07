use crate::suites::constants::BEACON_STRING;
use crate::suites::constants::CONSTANT_CONTENT_KEY;
use crate::suites::constants::CONSTANT_CONTENT_VALUE;
use crate::suites::constants::HIVE_PORTAL_NETWORKS_SELECTED;
use crate::suites::constants::TRIN_BRIDGE_CLIENT_TYPE;
use ethportal_api::types::enr::generate_random_remote_enr;
use ethportal_api::Discv5ApiClient;
use ethportal_api::{BeaconContentKey, BeaconNetworkApiClient};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use serde_json::json;
use std::collections::HashMap;

dyn_async! {
    pub async fn run_rpc_compat_test_suite<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        // Test single type of client
        for client in &clients {
            test.run(
                NClientTestSpec {
                    name: "discv5_nodeInfo".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_node_info,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLocalContent Expect ContentAbsent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_absent,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconStore".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_store,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLocalContent Expect ContentPresent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconAddEnr Expect true".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_add_enr_expect_true,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_local_enr,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconDeleteEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconDeleteEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_local_enr,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconRecursiveFindContent Content Absent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_recursive_find_content_content_absent,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), BEACON_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;
        }
    }
}

dyn_async! {
    async fn test_node_info<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };

        let response = Discv5ApiClient::node_info(&client.rpc).await;

        if let Err(err) = response {
            panic!("Expected response not received: {err}");
        }
    }
}

dyn_async! {
    async fn test_local_content_expect_content_absent<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key = serde_json::from_value(json!(CONSTANT_CONTENT_KEY));

        match content_key {
            Ok(content_key) => {
                if let Ok(response)  = BeaconNetworkApiClient::local_content(&client.rpc, content_key).await {
                    panic!("Expected to recieve an error because content wasn't found {response:?}");
                }
            }
            Err(err) => {
                panic!("{}", &err.to_string());
            }
        }
    }
}

dyn_async! {
    async fn test_store<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key =
        serde_json::from_value(json!(CONSTANT_CONTENT_KEY));

        let content_value =
        serde_json::from_value(json!(CONSTANT_CONTENT_VALUE));

        match content_key {
            Ok(content_key) => {
                match content_value {
                    Ok(content_value) => {
                        let response = BeaconNetworkApiClient::store(&client.rpc, content_key, content_value).await;

                        if let Err(err) = response {
                            panic!("{}", &err.to_string());
                        }
                    }
                    Err(err) => {
                        panic!("{}", &err.to_string());
                    }
                }
            }
            Err(err) => {
                panic!("{}", &err.to_string());
            }
        }
    }
}

dyn_async! {
    async fn test_local_content_expect_content_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key: Result<ethportal_api::BeaconContentKey, serde_json::Error> =
        serde_json::from_value(json!(CONSTANT_CONTENT_KEY));

        let content_value =
        serde_json::from_value(json!(CONSTANT_CONTENT_VALUE));


        match content_key {
            Ok(content_key) => {
                // seed CONTENT_KEY/content_value onto the local node to test local_content expect content present
                match content_value {
                    Ok(content_value) => {
                        let response = BeaconNetworkApiClient::store(&client.rpc, content_key.clone(), content_value).await;

                        if let Err(err) = response {
                            panic!("{}", &err.to_string());
                        }
                    }
                    Err(err) => {
                        panic!("{}", &err.to_string());
                    }
                }

                // Here we are calling local_content RPC to test if the content is present
                if let Err(err) = BeaconNetworkApiClient::local_content(&client.rpc, content_key).await {
                    panic!("Expected content returned from local_content to be present {}", &err.to_string());
                }
            }
            Err(err) => {
                panic!("{}", &err.to_string());
            }
        }
    }
}

dyn_async! {
    async fn test_add_enr_expect_true<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();
        match BeaconNetworkApiClient::add_enr(&client.rpc, enr).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_get_enr_non_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        if (BeaconNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("GetEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_get_enr_local_enr<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        // get our local enr from NodeInfo
        let target_enr = match Discv5ApiClient::node_info(&client.rpc).await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::get_enr(&client.rpc, target_enr.node_id()).await {
            Ok(response) => {
                if response != target_enr {
                    panic!("Response from GetEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_get_enr_enr_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match BeaconNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => {
                if response != enr {
                    panic!("Response from GetEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_delete_enr_non_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();
        match BeaconNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => match response {
                true => panic!("DeleteEnr expected to get false and instead got true"),
                false => ()
            },
            Err(err) => panic!("{}", &err.to_string()),
        };
    }
}

dyn_async! {
    async fn test_delete_enr_enr_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match BeaconNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if data was seeded into the table
        match BeaconNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => {
                if response != enr {
                    panic!("Response from GetEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // delete the data from routing table
        match BeaconNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("DeleteEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        };

        // check if the enr was actually deleted out of the table or not
        if (BeaconNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("GetEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_non_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        if (BeaconNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("LookupEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_enr_present<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match BeaconNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => {
                if response != enr {
                    panic!("Response from LookupEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_local_enr<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        // get our local enr from NodeInfo
        let target_enr = match Discv5ApiClient::node_info(&client.rpc).await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::lookup_enr(&client.rpc, target_enr.node_id()).await {
            Ok(response) => {
                if response != target_enr {
                    panic!("Response from LookupEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    // test that a node will return a AbsentContent via RecursiveFindContent when the data doesn't exist
    async fn test_recursive_find_content_content_absent<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let header_with_proof_key: BeaconContentKey = serde_json::from_value(json!(CONSTANT_CONTENT_KEY)).unwrap();

        if let Ok(content) = BeaconNetworkApiClient::recursive_find_content(&client.rpc, header_with_proof_key).await {
            panic!("Error: Unexpected RecursiveFindContent expected to not get the content and instead get an error: {content:?}");
        }
    }
}
