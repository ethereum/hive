use crate::suites::beacon::bridge::provider::ConsensusProvider;
use ethportal_api::utils::bytes::hex_decode;
use ethportal_api::BeaconContentValue;
use ethportal_api::BeaconNetworkApiClient;
use ethportal_api::ContentValue;
use hivesim::Client;
use std::sync::Arc;
use tokio::{
    sync::Mutex,
    time::{self, Duration},
};

// For the sync tests, we need fine-grained control over the data entering
// the network. The portal-bridge isn't great at providing this flexibility,
// so instead we have this mock bridge service that we can use to inject
// data into the network.
pub struct BridgeService {
    provider: Arc<ConsensusProvider>,
    query_interval: Duration,
    latest_optimistic_root: Arc<Mutex<Option<[u8; 32]>>>,
    latest_finalized_root: Arc<Mutex<Option<[u8; 32]>>>,
    trusted_block_root: Arc<Mutex<Option<[u8; 32]>>>,
    portal_client: Client,
}

impl BridgeService {
    pub fn new(portal_client: Client) -> Self {
        let provider = ConsensusProvider::new().unwrap();
        Self {
            provider: Arc::new(provider),
            query_interval: Duration::from_secs(3),
            portal_client,
            latest_optimistic_root: Arc::new(Mutex::new(None)),
            latest_finalized_root: Arc::new(Mutex::new(None)),
            trusted_block_root: Arc::new(Mutex::new(None)),
        }
    }

    pub async fn latest_optimistic_root(&self) -> Option<[u8; 32]> {
        *self.latest_optimistic_root.lock().await
    }

    pub async fn latest_finalized_root(&self) -> Option<[u8; 32]> {
        *self.latest_finalized_root.lock().await
    }

    pub async fn trusted_block_root(&self) -> Option<[u8; 32]> {
        *self.trusted_block_root.lock().await
    }

    pub async fn start(&self) {
        let provider = self.provider.clone();
        let query_interval = self.query_interval;
        let portal_client = self.portal_client.clone();
        let trusted_block_root_val = self.trusted_block_root.clone();
        let finality_update_val = self.latest_finalized_root.clone();
        let optimistic_update_val = self.latest_optimistic_root.clone();

        tokio::spawn(async move {
            let mut interval = time::interval(query_interval);

            loop {
                interval.tick().await;

                // fetch latest finalized root from provider and update the local value
                let trusted_block_root = provider.get_finalized_root().await.unwrap();
                {
                    let mut old_trusted_block_root = trusted_block_root_val.lock().await;
                    *old_trusted_block_root = hex_decode(&trusted_block_root)
                        .map(|bytes| bytes.try_into().ok())
                        .ok()
                        .flatten();
                }

                let data = provider
                    .get_light_client_bootstrap(trusted_block_root)
                    .await
                    .unwrap();
                let _ = portal_client
                    .rpc
                    .store(data.0.clone(), data.1.clone().encode())
                    .await;

                // fetch latest finality update from provider and seed it into portal_client
                let finality_update = provider.get_light_client_finality_update().await.unwrap();
                let _ = portal_client
                    .rpc
                    .store(
                        finality_update.0.clone(),
                        finality_update.1.clone().encode(),
                    )
                    .await;
                {
                    let mut old_finality_update = finality_update_val.lock().await;
                    *old_finality_update = match finality_update.1 {
                        BeaconContentValue::LightClientFinalityUpdate(update) => update
                            .update
                            .finalized_header_deneb()
                            .map(|header| header.beacon.state_root.0)
                            .ok(),
                        _ => panic!("Unexpected finality update content value"),
                    };
                }

                // fetch latest optimistic update from provider and seed it into portal_client
                let optimistic_update =
                    provider.get_light_client_optimistic_update().await.unwrap();
                let _ = portal_client
                    .rpc
                    .store(
                        optimistic_update.0.clone(),
                        optimistic_update.1.clone().encode(),
                    )
                    .await;
                {
                    let mut old_optimistic_update = optimistic_update_val.lock().await;
                    *old_optimistic_update = match optimistic_update.1 {
                        BeaconContentValue::LightClientOptimisticUpdate(update) => update
                            .update
                            .attested_header_deneb()
                            .map(|header| header.beacon.state_root.0)
                            .ok(),
                        _ => panic!("Unexpected optimistic update content value"),
                    };
                }
            }
        });
    }
}
