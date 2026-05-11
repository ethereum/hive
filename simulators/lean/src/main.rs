#![warn(clippy::unwrap_used)]

mod scenarios;
mod utils;

use crate::scenarios::client_interop::run_client_interop_lean_test_suite;
use crate::scenarios::gossip::run_gossip_lean_test_suite;
use crate::scenarios::reqresp::run_reqresp_lean_test_suite;
use crate::scenarios::rpc_compat::run_rpc_compat_lean_test_suite;
use crate::scenarios::spec_assets::{
    run_spec_assets_fork_choice_lean_test_suite, run_spec_assets_state_transition_lean_test_suite,
    run_spec_assets_verify_signatures_lean_test_suite,
};
use crate::scenarios::sync::run_sync_lean_test_suite;
use crate::scenarios::validation::run_validation_lean_test_suite;
use crate::utils::util::{resolve_selected_lean_devnet, set_selected_lean_devnet};
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
            "Runs Lean RPC compatibility tests against the selected lean clients using the {devnet} profile."
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
            "Runs Lean sync tests against the selected lean clients using the {devnet} profile."
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

    let mut client_interop = Suite {
        name: "client-interop".to_string(),
        description: format!(
            "Runs three-node Lean client interoperability tests across every selected client pair and each selected client by itself using the {devnet} profile."
        ),
        tests: vec![],
    };

    client_interop.add(TestSpec {
        name: "client-interop: matrix".to_string(),
        description:
            "Runs every selected Lean client against itself once and every other selected Lean client in both 2:1 topologies."
                .to_string(),
        always_run: true,
        run: run_client_interop_lean_test_suite,
        client: None,
    });

    let mut validation = Suite {
        name: "validation".to_string(),
        description: format!(
            "Runs Lean validation tests against the selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    validation.add(TestSpec {
        name: "validation: client launch".to_string(),
        description: "This test launches the client and runs validation scenarios.".to_string(),
        always_run: true,
        run: run_validation_lean_test_suite,
        client: None,
    });

    let mut gossip = Suite {
        name: "gossip".to_string(),
        description: format!(
            "Runs Lean gossipsub tests against the selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    gossip.add(TestSpec {
        name: "gossip: client launch".to_string(),
        description: "This test launches the client and runs gossip scenarios.".to_string(),
        always_run: true,
        run: run_gossip_lean_test_suite,
        client: None,
    });

    let mut reqresp = Suite {
        name: "reqresp".to_string(),
        description: format!(
            "Runs Lean req/resp protocol tests against the selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    reqresp.add(TestSpec {
        name: "reqresp: client launch".to_string(),
        description: "This test launches the client and runs req/resp scenarios.".to_string(),
        always_run: true,
        run: run_reqresp_lean_test_suite,
        client: None,
    });

    let mut spec_assets_fork_choice = Suite {
        name: "lean-spec-tests-fork-choice".to_string(),
        description: format!(
            "Runs generated Lean fork-choice spec-test fixtures against selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    spec_assets_fork_choice.add(TestSpec {
        name: "lean-spec-tests-fork-choice: fixture loader".to_string(),
        description:
            "Loads Lean fork-choice spec-test fixture files and runs each fixture as a distinct Hive test."
                .to_string(),
        always_run: true,
        run: run_spec_assets_fork_choice_lean_test_suite,
        client: None,
    });

    let mut spec_assets_state_transition = Suite {
        name: "lean-spec-tests-state-transition".to_string(),
        description: format!(
            "Runs generated Lean state-transition spec-test fixtures against selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    spec_assets_state_transition.add(TestSpec {
        name: "lean-spec-tests-state-transition: fixture loader".to_string(),
        description:
            "Loads Lean state-transition spec-test fixture files and runs each fixture as a distinct Hive test."
                .to_string(),
        always_run: true,
        run: run_spec_assets_state_transition_lean_test_suite,
        client: None,
    });

    let mut spec_assets_verify_signatures = Suite {
        name: "lean-spec-tests-verify-signatures".to_string(),
        description: format!(
            "Runs generated Lean signature-verification spec-test fixtures against selected lean clients using the {devnet} profile."
        ),
        tests: vec![],
    };

    spec_assets_verify_signatures.add(TestSpec {
        name: "lean-spec-tests-verify-signatures: fixture loader".to_string(),
        description:
            "Loads Lean signature-verification spec-test fixture files and runs each fixture as a distinct Hive test."
                .to_string(),
        always_run: true,
        run: run_spec_assets_verify_signatures_lean_test_suite,
        client: None,
    });

    run_suite(
        simulation,
        vec![
            rpc_compat,
            sync,
            client_interop,
            validation,
            gossip,
            reqresp,
            spec_assets_fork_choice,
            spec_assets_state_transition,
            spec_assets_verify_signatures,
        ],
    )
    .await;
}
