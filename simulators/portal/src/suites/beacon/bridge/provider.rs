use anyhow::{anyhow, Result};
use ethportal_api::{
    light_client::{
        bootstrap::LightClientBootstrapDeneb, finality_update::LightClientFinalityUpdateDeneb,
        optimistic_update::LightClientOptimisticUpdateDeneb,
    },
    types::content_key::beacon::{
        LightClientBootstrapKey, LightClientFinalityUpdateKey, LightClientOptimisticUpdateKey,
    },
    utils::bytes::hex_decode,
    BeaconContentKey, BeaconContentValue,
};
use reqwest::{
    header::{HeaderMap, HeaderValue, CONTENT_TYPE},
    Client,
};
use tracing::info;

// The consensus client for fetching data from the consensus layer, to feed
// into the test network.
pub struct ConsensusProvider {
    client: Client,
    base_url: String,
}

impl ConsensusProvider {
    pub fn new() -> Result<Self> {
        // check if the PORTAL_CONSENSUS_URL is set
        let base_url = match std::env::var("PORTAL_CONSENSUS_URL") {
            // trim trailing slash if present
            Ok(val) => val.trim_end_matches('/').to_string(),
            Err(_) => panic!("PORTAL_CONSENSUS_URL not set"),
        };
        info!("Beacon client initialized with base url: {base_url}");
        let mut headers = HeaderMap::new();
        headers.insert(CONTENT_TYPE, HeaderValue::from_static("application/json"));
        if base_url.contains("pandaops") {
            if let Ok(val) = std::env::var("PORTAL_CONSENSUS_AUTH") {
                let (client_id, client_secret) = val.split_once(":").expect(
                    "PORTAL_CONSENSUS_AUTH must be in the format 'client_id:client_secret'",
                );
                headers.insert("CF-Access-Client-ID", HeaderValue::from_str(client_id)?);
                headers.insert(
                    "CF-Access-Client-Secret",
                    HeaderValue::from_str(client_secret)?,
                );
            };
        }
        let client = Client::builder()
            .default_headers(headers)
            .build()
            .map_err(|_| anyhow!("Failed to build HTTP client"))?;
        Ok(Self { client, base_url })
    }

    pub async fn get_finalized_root(&self) -> Result<String> {
        info!("Fetching finalized root");
        let url = format!("{}/eth/v1/beacon/blocks/finalized/root", self.base_url);
        let data = make_request(&self.client, &url).await?;
        Ok(data["root"].as_str().unwrap().to_string())
    }

    pub async fn get_light_client_bootstrap(
        &self,
        block_root: String,
    ) -> Result<(BeaconContentKey, BeaconContentValue)> {
        info!("Fetching light client bootstrap data");
        let url = format!(
            "{}/eth/v1/beacon/light_client/bootstrap/{}",
            self.base_url, block_root
        );
        let data = make_request(&self.client, &url).await?;
        let content_key = BeaconContentKey::LightClientBootstrap(LightClientBootstrapKey {
            block_hash: <[u8; 32]>::try_from(hex_decode(&block_root)?).unwrap(),
        });
        let bootstrap: LightClientBootstrapDeneb = serde_json::from_value(data)?;
        let content_value = BeaconContentValue::LightClientBootstrap(bootstrap.into());

        Ok((content_key, content_value))
    }

    pub async fn get_light_client_finality_update(
        &self,
    ) -> Result<(BeaconContentKey, BeaconContentValue)> {
        info!("Fetching light client finality update");
        let url = format!(
            "{}/eth/v1/beacon/light_client/finality_update",
            self.base_url
        );
        let data = make_request(&self.client, &url).await?;
        let update: LightClientFinalityUpdateDeneb = serde_json::from_value(data)?;
        let new_finalized_slot = update.finalized_header.beacon.slot;
        let content_key = BeaconContentKey::LightClientFinalityUpdate(
            LightClientFinalityUpdateKey::new(new_finalized_slot),
        );
        let content_value = BeaconContentValue::LightClientFinalityUpdate(update.into());

        Ok((content_key, content_value))
    }

    pub async fn get_light_client_optimistic_update(
        &self,
    ) -> Result<(BeaconContentKey, BeaconContentValue)> {
        info!("Fetching light client optimistic update");
        let url = format!(
            "{}/eth/v1/beacon/light_client/optimistic_update",
            self.base_url
        );
        let data = make_request(&self.client, &url).await?;
        let update: LightClientOptimisticUpdateDeneb = serde_json::from_value(data)?;
        let content_key = BeaconContentKey::LightClientOptimisticUpdate(
            LightClientOptimisticUpdateKey::new(update.signature_slot),
        );
        let content_value = BeaconContentValue::LightClientOptimisticUpdate(update.into());

        Ok((content_key, content_value))
    }
}

async fn make_request(client: &Client, url: &str) -> Result<serde_json::Value> {
    let response = client.get(url).send().await?;
    let json_data = response
        .error_for_status()?
        .json::<serde_json::Value>()
        .await?;
    Ok(json_data["data"].clone())
}
