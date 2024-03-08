use crate::suites::constants::HIVE_PORTAL_NETWORKS_SELECTED;
use crate::suites::constants::STATE_STRING;
use crate::suites::constants::TRIN_BRIDGE_CLIENT_TYPE;
use ethportal_api::types::enr::generate_random_remote_enr;
use ethportal_api::Discv5ApiClient;
use ethportal_api::{StateContentKey, StateNetworkApiClient};
use hivesim::types::ClientDefinition;
use hivesim::types::TestData;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use serde_json::json;
use std::collections::HashMap;

// Bootstrap https://github.com/ethereum/portal-spec-tests/blob/master/tests/mainnet/state/validation/account_trie_node.yaml
const CONTENT_KEY: &str =
    "0x20240000000ad14c73a3b489e9cb1c523aef684ed17363e03d33345f2b23c0407f87ee3ff000a97f";
const CONTENT_VALUE: &str = "0x24000000d4e56740f876aef8c010b86a40d5f56745a118d0906a34e69aec8c0db1cb8fa314000000280200003c0400001006000033060000f90211a090dcaf88c40c7bbc95a912cbdde67c175767b31173df9ee4b0d733bfdd511c43a0babe369f6b12092f49181ae04ca173fb68d1a5456f18d20fa32cba73954052bda0473ecf8a7e36a829e75039a3b055e51b8332cbf03324ab4af2066bbd6fbf0021a0bbda34753d7aa6c38e603f360244e8f59611921d9e1f128372fec0d586d4f9e0a04e44caecff45c9891f74f6a2156735886eedf6f1a733628ebc802ec79d844648a0a5f3f2f7542148c973977c8a1e154c4300fec92f755f7846f1b734d3ab1d90e7a0e823850f50bf72baae9d1733a36a444ab65d0a6faaba404f0583ce0ca4dad92da0f7a00cbe7d4b30b11faea3ae61b7f1f2b315b61d9f6bd68bfe587ad0eeceb721a07117ef9fc932f1a88e908eaead8565c19b5645dc9e5b1b6e841c5edbdfd71681a069eb2de283f32c11f859d7bcf93da23990d3e662935ed4d6b39ce3673ec84472a0203d26456312bbc4da5cd293b75b840fc5045e493d6f904d180823ec22bfed8ea09287b5c21f2254af4e64fca76acc5cd87399c7f1ede818db4326c98ce2dc2208a06fc2d754e304c48ce6a517753c62b1a9c1d5925b89707486d7fc08919e0a94eca07b1c54f15e299bd58bdfef9741538c7828b5d7d11a489f9c20d052b3471df475a051f9dd3739a927c89e357580a4c97b40234aa01ed3d5e0390dc982a7975880a0a089d613f26159af43616fd9455bb461f4869bfede26f2130835ed067a8b967bfb80f90211a068c361d52a1ae36e33155313cd4e838c539d946d85925e5867b747e66995279ea04bfcad7bfdf0fb113e6bf0ebe4d55543688888bd6ad277de20aa0f8a85808808a086816b98cc35e76d6e811646f6c37a73d148a23420bcc03493090639bb8393eca0af4b0b8fcf790b63a8bbc3711f091758d3e4dd424cdc4ded526015c42ca50dcaa0158ecf58c068c4d0452dba821665bbdc6c9c57d6016e53abc18832e9ccfc0b86a016a6ebfa551718656d0b514f948a859964f83d3717e0f055f996772d2f228c2da0a9fd3ba226e84a434bc8d5b974b5131faccdefa6a58a02ddff829fa3d4bb27d5a0cf039eb80d70511d207a7e25160061689d9267630b66d2aaedac436872ae415ba0f24a488dd566079b384e558779c105af0d8f8c8ed564aeab9c7564f393bea56ea05132b24ec0dcf33ec24cb4678aa90b2622141c94345a5dceff21b22284c26251a0c093497e0aeeb4bd0bc461643cf003f87dde10cc1c9925a0b23a409f2048b8e2a0c03b6353e1dcdd3707caa20cd4d6a5c3c59da9f7424bba97ba643f17d42b8b86a0177fd57a9ee5d2d47d712d2daf75f5162a33b604b72766a98daad5509f607b45a09c7945f1f9517363d0cc19dde9ad91222f9d3c0162061182eb114e39a9f605c7a0a8440ef6e36d273b58c65344edab0583a03f09912d22e723dae51cc1d716d786a01df56337e1161f3fe7318ad10bd8e5c42f741c0652bd70d1292c7d0f2f7b88cb80f901d1a01a42abb27509a631169ab7e765e13dec0de47dac9cc7bba9cbc03d9086a0db71a0a0978af82352385b3ed1d0cd6c37a5b2613a989ea01df570edb06df32fdefaff80a0d1cf5bc45e21fa3960d172fe189be5978adab50ca77d11d1c2953eebbbd2d312a005b8054e0db723ad20cc31b1be4d987ae6c6c37c334207120b1f67506f84ad43a0430e5e56596211fcf7b7875da97ac1b1f6ba13189dce330bcaebb9e6d5569f01a003353e7e139443c7b994c347eb7636dcae5b3eac6ba09b3bbb8c588c2e41a958a0cf966488cfb696ed5ac8f67923e4c0912eef7656df7f7586ba2b7ad7072b34b6a0652fb2d2acc28fc9360ac63f29d7886abf602ccbc0cf865061b653415256f357a062d4f171bb087621727a87a01a29d4c8b20864fcb45eb6d60eb929af76aa3485a0dbf844359c229020360ae86751fed8ea41d985ee245da8a27893cc5e1cf55d6fa03ada43c4de6154b5f5567fc27f3a659c3028b321a8040c8713d8b347d7634d3080a0728693f4b5a3f28187456de3217e2985662d52979bfae5a49da65153c50ada8da03c0a8e099ac8060b14d7946c6abc5a0a428defe61781e2cb0736a20ad086e442a07ca441b942818197c8fa360f6d32113277c734eedac7a4539f62e5a27ce404e880e21fa00ad14c73a3b489e9cb1c523aef684ed17363e03d33345f2b23c0407f87ee3ff0f8518080808080808080a01908e7f8035023929a2fa13d9f801a42db49a3063a350da55dd94896a3e9be0a80808080a00cfd334f65fc252dbe0bfa903aaef7bb1546db01627d8eacd919c2bce43b6691808080";

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
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateLocalContent Expect ContentAbsent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_absent,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateStore".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_store,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateLocalContent Expect ContentPresent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_local_content_expect_content_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateAddEnr Expect true".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_add_enr_expect_true,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateGetEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateGetEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateGetEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_get_enr_local_enr,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateDeleteEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateDeleteEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_delete_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateLookupEnr None Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_non_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateLookupEnr ENR Found".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_enr_present,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateLookupEnr Local Enr".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_lookup_enr_local_enr,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;

            test.run(
                NClientTestSpec {
                    name: "portal_stateRecursiveFindContent Content Absent".to_string(),
                    description: "".to_string(),
                    always_run: false,
                    run: test_recursive_find_content_content_absent,
                    environments: Some(vec![Some(HashMap::from([(HIVE_PORTAL_NETWORKS_SELECTED.to_string(), STATE_STRING.to_string())]))]),
                    test_data: None,
                    clients: vec![client.clone()],
                }
            ).await;
        }
    }
}

dyn_async! {
    async fn test_node_info<'a>(clients: Vec<Client>, _: Option<TestData>) {
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
    async fn test_local_content_expect_content_absent<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key = serde_json::from_value(json!(CONTENT_KEY));

        match content_key {
            Ok(content_key) => {
                if let Ok(response)  = StateNetworkApiClient::local_content(&client.rpc, content_key).await {
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
    async fn test_store<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key =
        serde_json::from_value(json!(CONTENT_KEY));

        let content_value =
        serde_json::from_value(json!(CONTENT_VALUE));

        match content_key {
            Ok(content_key) => {
                match content_value {
                    Ok(content_value) => {
                        let response = StateNetworkApiClient::store(&client.rpc, content_key, content_value).await;

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
    async fn test_local_content_expect_content_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let content_key: Result<ethportal_api::StateContentKey, serde_json::Error> =
        serde_json::from_value(json!(CONTENT_KEY));

        let content_value =
        serde_json::from_value(json!(CONTENT_VALUE));


        match content_key {
            Ok(content_key) => {
                // seed content_key/content_value onto the local node to test local_content expect content present
                match content_value {
                    Ok(content_value) => {
                        let response = StateNetworkApiClient::store(&client.rpc, content_key.clone(), content_value).await;

                        if let Err(err) = response {
                            panic!("{}", &err.to_string());
                        }
                    }
                    Err(err) => {
                        panic!("{}", &err.to_string());
                    }
                }

                // Here we are calling local_content RPC to test if the content is present
                if let Err(err) = StateNetworkApiClient::local_content(&client.rpc, content_key).await {
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
    async fn test_add_enr_expect_true<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();
        match StateNetworkApiClient::add_enr(&client.rpc, enr).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }
    }
}

dyn_async! {
    async fn test_get_enr_non_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        if (StateNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("GetEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_get_enr_local_enr<'a>(clients: Vec<Client>, _: Option<TestData>) {
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
        match StateNetworkApiClient::get_enr(&client.rpc, target_enr.node_id()).await {
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
    async fn test_get_enr_enr_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match StateNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match StateNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
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
    async fn test_delete_enr_non_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();
        match StateNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => match response {
                true => panic!("DeleteEnr expected to get false and instead got true"),
                false => ()
            },
            Err(err) => panic!("{}", &err.to_string()),
        };
    }
}

dyn_async! {
    async fn test_delete_enr_enr_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match StateNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if data was seeded into the table
        match StateNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => {
                if response != enr {
                    panic!("Response from GetEnr didn't return expected Enr")
                }
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // delete the data from routing table
        match StateNetworkApiClient::delete_enr(&client.rpc, enr.node_id()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("DeleteEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        };

        // check if the enr was actually deleted out of the table or not
        if (StateNetworkApiClient::get_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("GetEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_non_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        if (StateNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await).is_ok() {
            panic!("LookupEnr in this case is not supposed to return a value")
        }
    }
}

dyn_async! {
    async fn test_lookup_enr_enr_present<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let (_, enr) = generate_random_remote_enr();

        // seed enr into routing table
        match StateNetworkApiClient::add_enr(&client.rpc, enr.clone()).await {
            Ok(response) => match response {
                true => (),
                false => panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        // check if we can fetch data from routing table
        match StateNetworkApiClient::lookup_enr(&client.rpc, enr.node_id()).await {
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
    async fn test_lookup_enr_local_enr<'a>(clients: Vec<Client>, _: Option<TestData>) {
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
        match StateNetworkApiClient::lookup_enr(&client.rpc, target_enr.node_id()).await {
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
    async fn test_recursive_find_content_content_absent<'a>(clients: Vec<Client>, _: Option<TestData>) {
        let client = match clients.into_iter().next() {
            Some((client)) => client,
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };
        let header_with_proof_key: StateContentKey = serde_json::from_value(json!(CONTENT_KEY)).unwrap();

        if let Ok(content) = StateNetworkApiClient::recursive_find_content(&client.rpc, header_with_proof_key).await {
            panic!("Error: Unexpected RecursiveFindContent expected to not get the content and instead get an error: {content:?}");
        }
    }
}
