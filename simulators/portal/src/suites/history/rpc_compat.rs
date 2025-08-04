use crate::suites::history::constants::{
    HEADER_WITH_PROOF_KEY, HEADER_WITH_PROOF_VALUE, TRIN_BRIDGE_CLIENT_TYPE,
};
use ethportal_api::types::enr::generate_random_remote_enr;
use ethportal_api::types::portal::GetContentInfo;
use ethportal_api::Discv5ApiClient;
use ethportal_api::HistoryNetworkApiClient;
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};

dyn_async! {
    pub async fn run_rpc_compat_history_test_suite<'a> (test: &'a mut Test, _client: Option<Client>) {
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
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyLocalContent Expect ContentAbsent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_absent,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyStore".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_store,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyLocalContent Expect ContentPresent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyAddEnr Expect true".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_add_enr_expect_true,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyGetEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_non_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyGetEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_enr_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyGetEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_local_enr,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyDeleteEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_non_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyDeleteEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_enr_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyLookupEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_non_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyLookupEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_enr_present,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyLookupEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_local_enr,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyGetContent Content Absent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_content_content_absent,
                    environments: None,
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_historyGetContent Content Present Locally".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_content_content_present_locally,
                    environments: None,
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
        let content_key = HEADER_WITH_PROOF_KEY.clone();

        if let Ok(response)  = HistoryNetworkApiClient::local_content(&client.rpc, content_key).await {
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
        let content_key = HEADER_WITH_PROOF_KEY.clone();
        let raw_content_value = HEADER_WITH_PROOF_VALUE.clone();

        if let Err(err) = HistoryNetworkApiClient::store(&client.rpc, content_key, raw_content_value).await {
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
        let content_key = HEADER_WITH_PROOF_KEY.clone();
        let raw_content_value = HEADER_WITH_PROOF_VALUE.clone();

        // seed content_key/content_value onto the local node to test local_content expect content present
        if let Err(err) = HistoryNetworkApiClient::store(&client.rpc, content_key.clone(), raw_content_value.clone()).await {
            panic!("Failed to store data: {err:?}");
        }

        // Here we are calling local_content RPC to test if the content is present
        match HistoryNetworkApiClient::local_content(&client.rpc, content_key).await {
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
        match HistoryNetworkApiClient::add_enr(&client.rpc, enr).await {
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

        if (HistoryNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
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
        match HistoryNetworkApiClient::get_enr(&client.rpc, target_enr.node_id()).await {
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
    async fn test_get_enr_enr_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match HistoryNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match HistoryNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
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
    async fn test_delete_enr_non_present<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let (_, enr) = generate_random_remote_enr();
        match HistoryNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => if response { panic!("DeleteEnr expected to get false and instead got true") },
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
        match HistoryNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if data was seeded into the table
        match HistoryNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => {
                if response != enr {
                    panic!("Response from GetEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // delete the data from routing table
        match HistoryNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("DeleteEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        };

        // check if the enr was actually deleted out of the table or not
        if (HistoryNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
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

        if (HistoryNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await).is_ok() {
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
        match HistoryNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match HistoryNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await {
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
        match HistoryNetworkApiClient::lookup_enr(&client.rpc, target_enr.node_id()).await {
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
    // test that a node will return a AbsentContent via GetContent when the data doesn't exist
    async fn test_get_content_content_absent<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => panic!("Unable to get expected amount of clients from NClientTestSpec"),
        };
        let header_with_proof_key = HEADER_WITH_PROOF_KEY.clone();

        if let Ok(content) = HistoryNetworkApiClient::get_content(&client.rpc, header_with_proof_key).await {
            panic!("Error: Unexpected GetContent expected to not get the content and instead get an error: {content:?}");
        }
    }
}

dyn_async! {
    // test that a node will return a PresentContent via GetContent when the data is stored locally
    async fn test_get_content_content_present_locally<'a>(clients: Vec<Client>, _: ()) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };


        let content_key = HEADER_WITH_PROOF_KEY.clone();
        let raw_content_value = HEADER_WITH_PROOF_VALUE.clone();

        // seed content_key/content_value onto the local node to test get_content expect content present
        if let Err(err) = HistoryNetworkApiClient::store(&client.rpc, content_key.clone(), raw_content_value.clone()).await {
            panic!("Failed to store data: {err:?}");
        }

        match HistoryNetworkApiClient::get_content(&client.rpc, content_key).await {
            Ok(GetContentInfo { content, utp_transfer }) => {
                assert!(!utp_transfer, "Error: Expected utp_transfer to be false");
                assert_eq!(content, raw_content_value, "Error receiving content");
            }
            Err(err) => panic!("Expected GetContent to not throw an error {err:?}"),
        }
    }
}
