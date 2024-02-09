mod suites;

use hivesim::{Simulation, Suite, TestSpec};
use suites::rpc_compat::run_rpc_compat_test_suite;

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let mut beacon_rpc_compat = Suite {
        name: "beacon-rpc-compat".to_string(),
        description: "The RPC-compatibility test suite runs a set of RPC related tests against a
        running node. It tests client implementations of the JSON-RPC API for
        conformance with the portal network API specification."
            .to_string(),
        tests: vec![],
    };

    beacon_rpc_compat.add(TestSpec {
        name: "client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: false,
        run: run_rpc_compat_test_suite,
        client: None,
    });

    let sim = Simulation::new();
    run_suite(sim, vec![beacon_rpc_compat]).await;
}

async fn run_suite(host: Simulation, suites: Vec<Suite>) {
    for suite in suites {
        let name = suite.clone().name;
        let description = suite.clone().description;

        let suite_id = host.start_suite(name, description, "".to_string()).await;

        for test in &suite.tests {
            test.run_test(host.clone(), suite_id, suite.clone()).await;
        }

        host.end_suite(suite_id).await;
    }
}
