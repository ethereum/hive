use std::collections::HashMap;
use std::str::FromStr;

use crate::suites::beacon::constants::{
    CONSTANT_CONTENT_KEY, CONSTANT_CONTENT_VALUE, TRIN_BRIDGE_CLIENT_TYPE,
};
use crate::suites::environment::PortalNetwork;
use alloy_primitives::Bytes;
use ethportal_api::types::enr::generate_random_remote_enr;
use ethportal_api::Discv5ApiClient;
use ethportal_api::{BeaconContentKey, BeaconNetworkApiClient};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use serde_json::json;

dyn_async! {
    pub async fn run_rpc_compat_beacon_test_suite<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let environment_flag = PortalNetwork::as_environment_flag([PortalNetwork::Beacon]);
        let environments = Some(vec![Some(HashMap::from([environment_flag]))]);

        // Test single type of client
        for client in &clients {
            test.run(
                NClientTestSpec {
                    name: "discv5_nodeInfo".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_node_info,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLocalContent Expect ContentAbsent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_absent,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconStore".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_store,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLocalContent Expect ContentPresent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconAddEnr Expect true".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_add_enr_expect_true,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_non_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_enr_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_local_enr,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconDeleteEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_non_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconDeleteEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_enr_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_non_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_enr_present,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconLookupEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_local_enr,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_beaconGetContent Content Absent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_content_content_absent,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;
        }
    }
}

dyn_async! {
    async fn test_node_info<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };

        if let Err(err) = Discv5ApiClient::node_info(&client.rpc).await {
            panic!("Expected response not received: {err}");
        }
    }
}

dyn_async! {
    async fn test_local_content_expect_content_absent<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let content_key: BeaconContentKey = serde_json::from_value(json!(CONSTANT_CONTENT_KEY)).unwrap();

        if let Ok(response)  = BeaconNetworkApiClient::local_content(&client.rpc, content_key).await {
            panic!("Expected to receive an error because content wasn't found {response:?}");
        }
    }
}

dyn_async! {
    async fn test_store<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let content_key: BeaconContentKey = serde_json::from_value(json!(CONSTANT_CONTENT_KEY)).unwrap();
        let raw_content_value = Bytes::from_str(CONSTANT_CONTENT_VALUE).expect("unable to convert content value to bytes");

        if let Err(err) = BeaconNetworkApiClient::store(&client.rpc, content_key, raw_content_value).await {
            panic!("{}", &err.to_string());
        }
    }
}

dyn_async! {
    async fn test_local_content_expect_content_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let content_key: BeaconContentKey = serde_json::from_value(json!(CONSTANT_CONTENT_KEY)).unwrap();
        let raw_content_value = Bytes::from_str(CONSTANT_CONTENT_VALUE).expect("unable to convert content value to bytes");

        // seed CONTENT_KEY/content_value onto the local node to test local_content expect content present
        if let Err(err) = BeaconNetworkApiClient::store(&client.rpc, content_key.clone(), raw_content_value.clone()).await {
            panic!("{}", &err.to_string());
        }

        // Here we are calling local_content RPC to test if the content is present
        match BeaconNetworkApiClient::local_content(&client.rpc, content_key).await {
            Ok(possible_content) => {
                if possible_content != raw_content_value {
                    panic!("Error receiving content: Expected content: {raw_content_value:?}, Received content: {possible_content:?}");
                }
            }
            Err(err) => panic!("Expected content returned from local_content to be present {}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_add_enr_expect_true<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
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
    async fn test_get_enr_non_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let (_, enr) = generate_random_remote_enr();

        if (BeaconNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("GetEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_get_enr_local_enr<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        // get our local enr from NodeInfo
        let target_enr = match Discv5ApiClient::node_info(&client.rpc).await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::get_enr(&client.rpc, target_enr.node_id()).await {
            Ok(response) => if response != target_enr {
                panic!("Response from GetEnr didn't return expected Enr")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_get_enr_enr_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
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
            Ok(response) => if response != enr {
                panic!("Response from GetEnr didn't return expected Enr");
            }
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_delete_enr_non_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
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
    async fn test_delete_enr_enr_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
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
            Ok(response) => if response != enr {
                panic!("Response from GetEnr didn't return expected Enr")
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
    async fn test_lookup_enr_non_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let (_, enr) = generate_random_remote_enr();

        if (BeaconNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("LookupEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_enr_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
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
            Ok(response) => if response != enr {
                panic!("Response from LookupEnr didn't return expected Enr")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_local_enr<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        // get our local enr from NodeInfo
        let target_enr = match Discv5ApiClient::node_info(&client.rpc).await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // check if we can fetch data from routing table
        match BeaconNetworkApiClient::lookup_enr(&client.rpc, target_enr.node_id()).await {
            Ok(response) => if response != target_enr {
                panic!("Response from LookupEnr didn't return expected Enr")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    // test that a node will return a AbsentContent via GetContent when the data doesn't exist
    async fn test_get_content_content_absent<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let header_with_proof_key: BeaconContentKey = serde_json::from_value(json!(CONSTANT_CONTENT_KEY)).unwrap();

        if let Ok(content) = BeaconNetworkApiClient::get_content(&client.rpc, header_with_proof_key).await {
            panic!("Error: Unexpected GetContent expected to not get the content and instead get an error: {content:?}");
        }
    }
}
