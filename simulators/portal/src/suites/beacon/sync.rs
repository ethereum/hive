use crate::suites::beacon::bridge::service::BridgeService;
use crate::suites::beacon::constants::{
    BOOTNODES_ENVIRONMENT_VARIABLE, TRUSTED_BLOCK_ROOT_ENVIRONMENT_VARIABLE,
};
use crate::suites::environment::PortalNetwork;
use ethportal_api::utils::bytes::hex_encode;
use ethportal_api::{BeaconNetworkApiClient, Discv5ApiClient};
use hivesim::{dyn_async, Client, NClientTestSpec, Test};
use std::collections::HashMap;
use std::sync::Arc;
use tokio::time::{sleep, Duration};

dyn_async! {
   pub async fn test_beacon_sync<'a> (test: &'a mut Test, _client: Option<Client>) {
        // Get all available portal clients
        let clients = test.sim.client_types().await;

        // this is the "blank" client, used just for storing beacon network
        // syncing data, and then we test sync functionality on the other client
        let environment = Some(HashMap::from([
            PortalNetwork::as_environment_flag([PortalNetwork::Beacon]),
        ]));
        let environments = Some(vec![environment.clone(), environment]);

        // Iterate over all possible pairings of clients and run the tests (including self-pairings)
        for client in &clients {
            test.run(
                NClientTestSpec {
                    name: format!("Beacon sync test: latest finalized root{}", client.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_client_syncs_with_latest_finalized_root,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;
            test.run(
                NClientTestSpec {
                    name: format!("Beacon sync test: latest optimistic root. {}", client.name),
                    description: "".to_string(),
                    always_run: false,
                    run: test_client_syncs_with_latest_optimistic_root,
                    environments: environments.clone(),
                    test_data: (),
                    clients: vec![client.clone()],
                }
            ).await;
        }
   }
}

dyn_async! {
    async fn test_client_syncs_with_latest_finalized_root<'a>(clients: Vec<Client>, _: ()) {
        let Some((client)) = clients.into_iter().next() else {
            panic!("Unable to get expected amount of clients from NClientTestSpec")
        };

        // starts the bridge service:
        // the bridge service acts like a beacon bridge, but simply injects
        // the latest data into the given client, so that it will be available
        // for syncing from the client we're testing
        let bridge_service = Arc::new(BridgeService::new(client.clone()));
        let service_for_task = bridge_service.clone();
        let provider_handle = tokio::spawn(async move {
            service_for_task.start().await;
        });

        // get enr
        let client_enr = match client.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // wait for initial trusted block root
        let mut trusted_block_root = None;
        for i in 0..30 {
            if trusted_block_root.is_some() {
                break;
            }
            if i == 29 {
                drop(provider_handle);
                panic!("Trusted block root not initialized in time");
            }
            sleep(Duration::from_millis(1000)).await;
            trusted_block_root = bridge_service.trusted_block_root().await;
        }

        // start the client that we're using to test syncing functionality
        let test_client = client.test.start_client(
            client.kind,
            Some(HashMap::from([
                (BOOTNODES_ENVIRONMENT_VARIABLE.to_string(), client_enr.to_base64()),
                (TRUSTED_BLOCK_ROOT_ENVIRONMENT_VARIABLE.to_string(), hex_encode(trusted_block_root.unwrap())),
                PortalNetwork::as_environment_flag([PortalNetwork::Beacon]),
            ]))).await;


        // sleep 1 second to allow client to start
        sleep(Duration::from_millis(1000)).await;

        // loop until beacon client is initialized
        for i in 0..30 {
            sleep(Duration::from_millis(1000)).await;
            if i == 29 {
                drop(provider_handle);
                panic!("Beacon client did not sync in time");
            }
            match test_client.rpc.finalized_state_root().await {
                Ok(val) => {
                    let actual_finalized_root = bridge_service.latest_finalized_root().await.unwrap();
                    assert_eq!(val, actual_finalized_root);
                    break;
                }
                Err(err) => {
                    // if error says "not initialized" then continue, since this indicates the client
                    // is not yet synced
                    if !err.to_string().contains("not initialized") {
                        drop(provider_handle);
                        panic!("Error getting finalized state root: {err:?}");
                    }
                }
            }
        }
        drop(provider_handle);
    }
}

dyn_async! {
    async fn test_client_syncs_with_latest_optimistic_root<'a>(clients: Vec<Client>, _: ()) {
        let Some((client)) = clients.into_iter().next() else {
            panic!("Unable to get expected amount of clients from NClientTestSpec")
        };

        // start bridge service
        // the bridge service acts like a beacon bridge, but simply injects
        // the latest data into the given client, so that it will be available
        // for syncing from the client we're testing
        let bridge_service = Arc::new(BridgeService::new(client.clone()));
        let service_for_task = bridge_service.clone();
        let provider_handle = tokio::spawn(async move {
            service_for_task.start().await;
        });

        // get enr
        let client_enr = match client.rpc.node_info().await {
            Ok(node_info) => node_info.enr,
            Err(err) => panic!("Error getting node info: {err:?}"),
        };

        // wait for initial trusted block root
        let mut trusted_block_root = None;
        for i in 0..30 {
            if trusted_block_root.is_some() {
                break;
            }
            if i == 29 {
                drop(provider_handle);
                panic!("Trusted block root not initialized in time");
            }
            sleep(Duration::from_millis(1000)).await;
            trusted_block_root = bridge_service.trusted_block_root().await;
        }

        // start the client that we're using to test syncing functionality
        let test_client = client.test.start_client(
            client.kind,
            Some(HashMap::from([
                (BOOTNODES_ENVIRONMENT_VARIABLE.to_string(), client_enr.to_base64()),
                (TRUSTED_BLOCK_ROOT_ENVIRONMENT_VARIABLE.to_string(), hex_encode(trusted_block_root.unwrap())),
                PortalNetwork::as_environment_flag([PortalNetwork::Beacon]),
            ]))).await;


        // sleep 1 second to allow client to start
        sleep(Duration::from_millis(1000)).await;

        // loop until beacon client is initialized
        for i in 0..30 {
            sleep(Duration::from_millis(1000)).await;
            if i == 29 {
                drop(provider_handle);
                panic!("Beacon client did not sync in time");
            }
            match test_client.rpc.optimistic_state_root().await {
                Ok(val) => {
                    let actual_optimistic_root = bridge_service.latest_optimistic_root().await.unwrap();
                    assert_eq!(val, actual_optimistic_root);
                    break;
                }
                Err(err) => {
                    // if error says "not initialized" then continue, since this indicates the client
                    // is not yet synced
                    if !err.to_string().contains("not initialized") {
                        drop(provider_handle);
                        panic!("Error getting optimistic state root: {err:?}");
                    }
                }
            }
        }
        drop(provider_handle);
    }
}
