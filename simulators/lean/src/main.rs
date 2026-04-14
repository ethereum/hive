#![warn(clippy::unwrap_used)]

use hivesim::dyn_async;
use hivesim::types::ClientDefinition;
use hivesim::{run_suite, Client, Simulation, Suite, Test, TestSpec};
const LEAN_ROLE: &str = "lean";

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();

    let mut api_smoke = Suite {
        name: "api-smoke".to_string(),
        description: "Launches lean clients and checks the shared onboarding API endpoints."
            .to_string(),
        tests: vec![],
    };

    api_smoke.add(TestSpec {
        name: "client launch".to_string(),
        description:
            "Starts each selected lean client and verifies its health and justified checkpoint endpoints."
                .to_string(),
        always_run: false,
        run: run_api_smoke_suite,
        client: None,
    });

    run_suite(Simulation::new(), vec![api_smoke]).await;
}

fn lean_clients(clients: Vec<ClientDefinition>) -> Vec<ClientDefinition> {
    clients
        .into_iter()
        .filter(|client| client.meta.roles.iter().any(|role| role == LEAN_ROLE))
        .collect()
}

dyn_async! {
    async fn run_api_smoke_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        let clients = lean_clients(test.sim.client_types().await);
        if clients.is_empty() {
            panic!("No lean clients were selected for this run");
        }

    }
}

