use std::fmt::Display;

use itertools::Itertools;

pub const HIVE_PORTAL_NETWORKS_SELECTED: &str = "HIVE_PORTAL_NETWORKS_SELECTED";

#[derive(Debug, Clone, Copy)]
pub enum PortalNetwork {
    History,
    Beacon,
    State,
}

impl PortalNetwork {
    pub fn as_environment_flag(
        networks: impl IntoIterator<Item = PortalNetwork>,
    ) -> (String, String) {
        let joined = networks
            .into_iter()
            .map(|network| network.to_string())
            .join(",");
        (HIVE_PORTAL_NETWORKS_SELECTED.to_string(), joined)
    }
}

impl Display for PortalNetwork {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            PortalNetwork::History => f.write_str("history"),
            PortalNetwork::Beacon => f.write_str("beacon"),
            PortalNetwork::State => f.write_str("state"),
        }
    }
}
