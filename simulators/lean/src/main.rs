#![warn(clippy::unwrap_used)]

mod scenarios;

use crate::scenarios::helper::{resolve_selected_lean_devnet, set_selected_lean_devnet};
use crate::scenarios::rpc_compat::run_rpc_compat_lean_test_suite;
use crate::scenarios::sync::run_sync_lean_test_suite;
use hivesim::{run_suite, Simulation, Suite, TestSpec};

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let simulation = Simulation::new();
    let devnet = resolve_selected_lean_devnet(&simulation).await;
    set_selected_lean_devnet(devnet);

    let mut rpc_compat = Suite {
        name: "rpc-compat".to_string(),
        description: format!(
            "Runs Lean RPC compatibility tests against the selected lean clients using the {} profile.",
            devnet
        ),
        tests: vec![],
    };

    rpc_compat.add(TestSpec {
        name: "rpc-compat: client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: true,
        run: run_rpc_compat_lean_test_suite,
        client: None,
    });

    let mut sync = Suite {
        name: "sync".to_string(),
        description: format!(
            "Runs Lean sync tests against the selected lean clients using the {} profile.",
            devnet
        ),
        tests: vec![],
    };

    sync.add(TestSpec {
        name: "sync: client launch".to_string(),
        description: "This test launches the client and collects its logs.".to_string(),
        always_run: true,
        run: run_sync_lean_test_suite,
        client: None,
    });

    run_suite(simulation, vec![rpc_compat, sync]).await;
}
