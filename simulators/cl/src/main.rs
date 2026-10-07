#![warn(clippy::unwrap_used)]

mod junit;
mod meta;
mod scenario;

use hivesim::{run_suite, PlannedTestSpec, Simulation, Suite};

#[tokio::main]
async fn main() {
    tracing_subscriber::fmt::init();
    let simulation = Simulation::new();

    let mut suite = Suite {
        name: "cl".to_string(),
        description: "Runs ethereum/consensus-spec-tests against each selected CL client \
             using its native test runner. Each client is built from source inside \
             a `<client>-specs` image whose entrypoint runs the client's own \
             spec-test suite and serves the resulting JUnit XML over HTTP for \
             this simulator to harvest."
            .to_string(),
        tests: vec![],
    };

    suite.add(PlannedTestSpec {
        name: "spec-tests".to_string(),
        description: "Per selected `cl-spec-runner` client, build from source, run \
             consensus-spec-tests with its native test runner, harvest the \
             JUnit XML, and emit one hive test per client."
            .to_string(),
        always_run: true,
        run: scenario::run_spec_tests,
        client: None,
    });

    run_suite(simulation, vec![suite]).await;
}
