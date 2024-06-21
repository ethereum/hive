mod suites;

use hivesim::{run_suite, Simulation, Suite, TestSpec};
use suites::interop::test_portal_interop;
use suites::rpc_compat::run_rpc_compat_test_suite;

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let mut state_rpc_compat = Suite {
        name: "state-rpc-compat".to_string(),
        description: "The RPC-compatibility test suite runs a set of RPC related tests against a
        running node. It tests client implementations of the JSON-RPC API for
        conformance with the portal network API specification."
            .to_string(),
        tests: vec![],
    };

    state_rpc_compat.add(TestSpec {
        name: "client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: false,
        run: run_rpc_compat_test_suite,
        client: None,
    });

    let mut interop = Suite {
        name: "state-interop".to_string(),
        description:
            "The interop test suite runs a set of scenarios to test interoperability between
        portal network clients"
                .to_string(),
        tests: vec![],
    };

    interop.add(TestSpec {
        name: "client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: false,
        run: test_portal_interop,
        client: None,
    });

    let sim = Simulation::new();
    run_suite(sim, vec![state_rpc_compat, interop]).await;
}
