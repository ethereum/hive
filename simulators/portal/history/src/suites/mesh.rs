use crate::suites::constants::TRIN_BRIDGE_CLIENT_TYPE;
use ethportal_api::jsonrpsee::core::__reexports::serde_json;
use ethportal_api::types::distance::{Metric, XorMetric};
use ethportal_api::types::portal::ContentInfo;
use ethportal_api::{
    Discv5ApiClient, HistoryContentKey, HistoryContentValue, HistoryNetworkApiClient,
};
use hivesim::types::ClientDefinition;
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use itertools::Itertools;
use serde_json::json;
use std::collections::HashMap;

// Header with proof for block number 14764013
const HEADER_WITH_PROOF_KEY: &str =
    "0x00720704f3aa11c53cf344ea069db95cecb81ad7453c8f276b2a1062979611f09c";
const HEADER_WITH_PROOF_VALUE: &str = "0x080000002d020000f90222a02c58e3212c085178dbb1277e2f3c24b3f451267a75a234945c1581af639f4a7aa058a694212e0416353a4d3865ccf475496b55af3a3d3b002057000741af9731919400192fb10df37c9fb26829eb2cc623cd1bf599e8a067a9fb631f4579f9015ef3c6f1f3830dfa2dc08afe156f750e90022134b9ebf6a018a2978fc62cd1a23e90de920af68c0c3af3330327927cda4c005faccefb5ce7a0168a3827607627e781941dc777737fc4b6beb69a8b139240b881992b35b854eab9010000200000400000001000400080080000000000010004010001000008000000002000110000000000000090020001110402008000080208040010000000a8000000000000000000210822000900205020000000000160020020000400800040000000000042080000000400004008084020001000001004004000001000000000000001000000110000040000010200844040048101000008002000404810082002800000108020000200408008000100000000000000002020000b00010080600902000200000050000400000000000000400000002002101000000a00002000003420000800400000020100002000000000000000c00040000001000000100187327bd7ad3116ce83e147ed8401c9c36483140db184627d9afa9a457468657265756d50504c4e532f326d696e6572735f55534133a0f1a32e24eb62f01ec3f2b3b5893f7be9062fbf5482bc0d490a54352240350e26882087fbb243327696851aae1651b6010cc53ffa2df1bae1550a0000000000000000000000000000000000000000000063d45d0a2242d35484f289108b3c80cccf943005db0db6c67ffea4c4a47fd529f64d74fa6068a3fd89a2c0d9938c3a751c4706d0b0e8f99dec6b517cf12809cb413795c8c678b3171303ddce2fa1a91af6a0961b9db72750d4d5ea7d5103d8d25f23f522d9af4c13fe8ac7a7d9d64bb08d980281eea5298b93cb1085fedc19d4c60afdd52d116cfad030cf4223e50afa8031154a2263c76eb08b96b5b8fdf5e5c30825d5c918eefb89daaf0e8573f20643614d9843a1817b6186074e4e53b22cf49046d977c901ec00aef1555fa89468adc2a51a081f186c995153d1cba0f2887d585212d68be4b958d309fbe611abe98a9bfc3f4b7a7b72bb881b888d89a04ecfe08b1c1a48554a48328646e4f864fe722f12d850f0be29e3829d1f94b34083032a9b6f43abd559785c996229f8e022d4cd6dcde4aafcce6445fe8743e1fcbe8672a99f9d9e3a5ca10c01f3751d69fbd22197f0680bc1529151130b22759bf185f4dbce357f46eb9cc8e21ea78f49b298eea2756d761fe23de8bea0d2e15aed136d689f6d252c54ebadc3e46b84a397b681edf7ec63522b9a298301084d019d0020000000000000000000000000000000000000000000000000000000000000";

// private key hive environment variable
const PRIVATE_KEY_ENVIRONMENT_VARIABLE: &str = "HIVE_CLIENT_PRIVATE_KEY";

dyn_async! {
   pub async fn test_portal_scenarios<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;
        // todo: remove this once we implement role in hivesim-rs
        let clients: Vec<ClientDefinition> = clients.into_iter().filter(|client| client.name != *TRIN_BRIDGE_CLIENT_TYPE).collect();

        let private_key_1 = "fc34e57cc83ed45aae140152fd84e2c21d1f4d46e19452e13acc7ee90daa5bac".to_string();
        let private_key_2 = "e5add57dc4c9ef382509e61ce106ec86f60eb73bbfe326b00f54bf8e1819ba11".to_string();

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for ((client_a, client_b), client_c) in clients.iter().cartesian_product(clients.iter()).cartesian_product(clients.iter()) {
            test.run(
                NClientTestSpec {
                    name: format!("FIND_CONTENT content stored 2 nodes away stored in client C (Client B closer to content then C). A:{} --> B:{} --> C:{}", client_a.name, client_b.name, client_c.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_find_content_two_jumps,
                    environments: Some(vec![None, Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_2.clone())])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_1.clone())]))]),
                    test_data: None,
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
                    environments: Some(vec![None, Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_1.clone())])), Some(HashMap::from([(PRIVATE_KEY_ENVIRONMENT_VARIABLE.to_string(), private_key_2.clone())]))]),
                    test_data: None,
                    clients: vec![client_a.clone(), client_b.clone(), client_c.clone()],
                }
            ).await;

            // Test find nodes distance of client a
            test.run(NClientTestSpec {
                    name: format!("FIND_NODES distance of client C {} --> {} --> {}", client_a.name, client_b.name, client_c.name),
                    description: "find nodes: distance of client A expect seeded enr returned".to_string(),
                    always_run: false,
                    run: test_find_nodes_distance_of_client_c,
                    environments: None,
                    test_data: None,
                    clients: vec![client_a.clone(), client_b.clone(), client_c.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    async fn test_find_content_two_jumps<'a> (clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let (client_a, client_b, client_c) = match clients.iter().collect_tuple() {
            Some((client_a, client_b, client_c)) => (client_a, client_b, client_c),
            None => {
                panic!("Unable to get expected amount of clients from NClientTestSpec");
            }
        };

        let header_with_proof_key: HistoryContentKey = serde_json::from_value(json!(HEADER_WITH_PROOF_KEY)).unwrap();
        let header_with_proof_value: HistoryContentValue = serde_json::from_value(json!(HEADER_WITH_PROOF_VALUE)).unwrap();

        // get enr for b and c to seed for the jumps
        let client_b_enr = match client_b.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        let client_c_enr = match client_c.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
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
        if let Err(err) = client_b.rpc.ping(client_c_enr.clone()).await {
                panic!("Unable to receive pong info: {err:?}");
        }

        // seed the data into client_c
        match client_c.rpc.store(header_with_proof_key.clone(), header_with_proof_value.clone()).await {
            Ok(result) => if !result {
                panic!("Unable to store header with proof for find content immediate return test");
            },
            Err(err) => {
                panic!("Error storing header with proof for find content immediate return test: {err:?}");
            }
        }

        let enrs = match client_a.rpc.find_content(client_b_enr.clone(), header_with_proof_key.clone()).await {
            Ok(result) => {
                match result {
                    ContentInfo::Enrs{ enrs } => {
                        enrs
                    },
                    other => {
                        panic!("Error: (Enrs) Unexpected FINDCONTENT response not: {other:?}");
                    }
                }
            },
            Err(err) => {
                panic!("Error: (Enrs) Unable to get response from FINDCONTENT request: {err:?}");
            }
        };

        if enrs.len() != 1 {
            panic!("Known node is closer to content, Enrs returned should be 0 instead got: length {}", enrs.len());
        }

        match client_a.rpc.find_content(enrs[0].clone(), header_with_proof_key.clone()).await {
            Ok(result) => {
                match result {
                    ContentInfo::Content{ content: ethportal_api::PossibleHistoryContentValue::ContentPresent(val), utp_transfer } => {
                        if val != header_with_proof_value {
                            panic!("Error: Unexpected FINDCONTENT response: didn't return expected header with proof value");
                        }

                        if utp_transfer {
                            panic!("Error: Unexpected FINDCONTENT response: utp_transfer was supposed to be false");
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
    async fn test_find_nodes_distance_of_client_c<'a>(clients: Vec<Client>, _: Option<Vec<(String, String)>>) {
        let (client_a, client_b, client_c) = match clients.iter().collect_tuple() {
            Some((client_a, client_b, client_c)) => (client_a, client_b, client_c),
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

        // We are adding client C to our list so we then can assume only one client per bucket
        let client_c_enr = match client_c.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => {
                panic!("Error getting node info: {err:?}");
            }
        };

        // seed enr into routing table
        match HistoryNetworkApiClient::add_enr(&client_b.rpc, client_c_enr.clone()).await {
            Ok(response) => if !response {
                panic!("AddEnr expected to get true and instead got false")
            },
            Err(err) => panic!("{}", &err.to_string()),
        }

        if let Some(distance) = XorMetric::distance(&target_enr.node_id().raw(), &client_c_enr.node_id().raw()).log2() {
            match client_a.rpc.find_nodes(target_enr.clone(), vec![distance as u16]).await {
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
