use crate::types::{ClientDefinition, SuiteID, TestID, TestResult};
use crate::Simulation;
use ::std::{boxed::Box, future::Future, pin::Pin};
use async_trait::async_trait;
use core::fmt::Debug;
use dyn_clone::DynClone;
use jsonrpsee::http_client::{HttpClient, HttpClientBuilder};
use std::collections::HashMap;
use std::net::IpAddr;

use crate::utils::extract_test_results;

pub type AsyncTestFunc = fn(
    &mut Test,
    Option<Client>,
) -> Pin<
    Box<
        dyn Future<Output = ()> // future API / pollable
            + Send // required by non-single-threaded executors
            + '_,
    >,
>;

pub type AsyncNClientsTestFunc<T> = fn(
    Vec<Client>,
    T,
) -> Pin<
    Box<
        dyn Future<Output = ()> // future API / pollable
            + Send // required by non-single-threaded executors
            + 'static,
    >,
>;

/// Signature for a scenario inside a `SharedClientTestSpec`. Each scenario gets
/// a shared, already-started `Client` plus a cloned `test_data` payload.
pub type AsyncSharedClientTestFunc<T> = fn(
    Client,
    T,
) -> Pin<
    Box<
        dyn Future<Output = ()> // future API / pollable
            + Send // required by non-single-threaded executors
            + 'static,
    >,
>;

type ClientFiles = Option<Vec<Option<HashMap<String, Vec<u8>>>>>;
type ClientEnvironments = Option<Vec<Option<HashMap<String, String>>>>;

#[async_trait]
pub trait Testable: DynClone + Send + Sync {
    async fn run_test(&self, simulation: Simulation, suite_id: SuiteID, suite: Suite);
}

impl Debug for dyn Testable {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        write!(f, "Testable")
    }
}

dyn_clone::clone_trait_object!(Testable);
/// Description of a test suite
#[derive(Clone, Debug)]
pub struct Suite {
    pub name: String,
    pub description: String,
    pub tests: Vec<Box<dyn Testable>>,
}

impl Suite {
    pub fn add<T: Testable + 'static>(&mut self, test: T) {
        self.tests.push(Box::new(test))
    }
}

/// Represents a running client.
#[derive(Debug, Clone)]
pub struct Client {
    pub kind: String,
    pub container: String,
    pub ip: IpAddr,
    pub rpc: HttpClient,
    pub test: Test,
}

#[derive(Clone, Debug)]
pub struct TestRun {
    pub suite_id: SuiteID,
    pub suite: Suite,
    pub name: String,
    pub desc: String,
    pub always_run: bool,
}

/// A running test
#[derive(Clone, Debug)]
pub struct Test {
    pub sim: Simulation,
    pub test_id: TestID,
    pub suite: Suite,
    pub suite_id: SuiteID,
    pub result: TestResult,
}

impl Test {
    pub async fn start_client(
        &self,
        client_type: String,
        environment: Option<HashMap<String, String>>,
    ) -> Client {
        self.start_client_with_files(client_type, environment, None)
            .await
    }

    pub async fn start_client_with_files(
        &self,
        client_type: String,
        environment: Option<HashMap<String, String>>,
        files: Option<HashMap<String, Vec<u8>>>,
    ) -> Client {
        let (container, ip) = self
            .sim
            .start_client_with_files(
                self.suite_id,
                self.test_id,
                client_type.clone(),
                environment,
                files,
            )
            .await;

        let rpc_url = format!("http://{}:8545", ip);

        let rpc_client = HttpClientBuilder::default()
            .build(rpc_url)
            .expect("Failed to build rpc_client");

        Client {
            kind: client_type,
            container,
            ip,
            rpc: rpc_client,
            test: Test {
                sim: self.sim.clone(),
                test_id: self.test_id,
                suite: self.suite.clone(),
                suite_id: self.suite_id,
                result: self.result.clone(),
            },
        }
    }

    /// Runs a subtest of this test.
    pub async fn run(&self, spec: impl Testable) {
        spec.run_test(self.sim.clone(), self.suite_id, self.suite.clone())
            .await
    }
}

#[derive(Clone)]
pub struct TestSpec {
    // These fields are displayed in the UI. Be sure to add
    // a meaningful description here.
    pub name: String,
    pub description: String,
    // If AlwaysRun is true, the test will run even if Name does not match the test
    // pattern. This option is useful for tests that launch a client instance and
    // then perform further tests against it.
    pub always_run: bool,
    // The Run function is invoked when the test executes.
    pub run: AsyncTestFunc,
    pub client: Option<Client>,
}

#[async_trait]
impl Testable for TestSpec {
    async fn run_test(&self, simulation: Simulation, suite_id: SuiteID, suite: Suite) {
        if let Some(test_match) = simulation.test_matcher.clone() {
            if !self.always_run && !test_match.match_test(&suite.name, &self.name) {
                return;
            }
        }

        let test_run = TestRun {
            suite_id,
            suite,
            name: self.name.to_owned(),
            desc: self.description.to_owned(),
            always_run: self.always_run,
        };

        run_test(simulation, test_run, self.client.clone(), self.run).await;
    }
}

pub async fn run_test(
    host: Simulation,
    test: TestRun,
    client: Option<Client>,
    func: AsyncTestFunc,
) {
    // Register test on simulation server and initialize the T.
    let test_id = host.start_test(test.suite_id, test.name, test.desc).await;
    let suite_id = test.suite_id;

    // run test function
    let cloned_host = host.clone();

    let test_result = extract_test_results(
        tokio::spawn(async move {
            let test = &mut Test {
                sim: cloned_host,
                test_id,
                suite: test.suite,
                suite_id,
                result: Default::default(),
            };

            test.result.pass = true;

            // run test function
            (func)(test, client).await;
        })
        .await,
    );

    host.end_test(suite_id, test_id, test_result).await;
}

#[derive(Clone)]
pub struct NClientTestSpec<T> {
    /// These fields are displayed in the UI. Be sure to add
    /// a meaningful description here.
    pub name: String,
    pub description: String,
    /// If AlwaysRun is true, the test will run even if Name does not match the test
    /// pattern. This option is useful for tests that launch a client instance and
    /// then perform further tests against it.
    pub always_run: bool,
    /// The Run function is invoked when the test executes.
    pub run: AsyncNClientsTestFunc<T>,
    /// For each client, there is a distinct map of Hive Environment Variable names to values.
    /// The environments must be in the same order as the `clients`
    pub environments: ClientEnvironments,
    /// For each client, there is a distinct map of destination file paths to file contents.
    /// The file maps must be in the same order as the `clients`.
    pub files: ClientFiles,
    /// test data which can be passed to the test
    pub test_data: T,
    pub clients: Vec<ClientDefinition>,
}

#[async_trait]
impl<T: Clone + Send + Sync + 'static> Testable for NClientTestSpec<T> {
    async fn run_test(&self, simulation: Simulation, suite_id: SuiteID, suite: Suite) {
        if let Some(test_match) = simulation.test_matcher.clone() {
            if !self.always_run && !test_match.match_test(&suite.name, &self.name) {
                return;
            }
        }

        let test_run = TestRun {
            suite_id,
            suite,
            name: self.name.to_owned(),
            desc: self.description.to_owned(),
            always_run: self.always_run,
        };

        run_n_client_test(
            simulation,
            test_run,
            self.environments.to_owned(),
            self.files.to_owned(),
            self.test_data.clone(),
            self.clients.to_owned(),
            self.run,
        )
        .await;
    }
}

// Write a test that runs against N clients.
async fn run_n_client_test<T: Send + 'static>(
    host: Simulation,
    test: TestRun,
    environments: ClientEnvironments,
    files: ClientFiles,
    test_data: T,
    clients: Vec<ClientDefinition>,
    func: AsyncNClientsTestFunc<T>,
) {
    // Register test on simulation server and initialize the T.
    let test_id = host.start_test(test.suite_id, test.name, test.desc).await;
    let suite_id = test.suite_id;

    // run test function
    let cloned_host = host.clone();
    let test_result = extract_test_results(
        tokio::spawn(async move {
            let test = &mut Test {
                sim: cloned_host,
                test_id,
                suite: test.suite,
                suite_id,
                result: Default::default(),
            };

            test.result.pass = true;

            let mut client_vec: Vec<Client> = Vec::new();
            let env_iter = environments.unwrap_or(vec![None; clients.len()]);
            let file_iter = files.unwrap_or(vec![None; clients.len()]);
            for ((client, environment), files) in clients.into_iter().zip(env_iter).zip(file_iter) {
                client_vec.push(
                    test.start_client_with_files(client.name.to_owned(), environment, files)
                        .await,
                );
            }
            (func)(client_vec, test_data).await;
        })
        .await,
    );

    host.end_test(suite_id, test_id, test_result).await;
}

/// One scenario inside a [`SharedClientTestSpec`].
///
/// Each scenario is surfaced as its own hive test case (pass/fail/details are
/// recorded per-scenario and shown individually in hiveview), but all scenarios
/// in the parent spec run against the same client container without
/// restarting it between scenarios.
#[derive(Clone)]
pub struct SharedClientScenario<T> {
    pub name: String,
    pub description: String,
    /// If true, the scenario runs even when the test matcher would otherwise
    /// filter it out.
    pub always_run: bool,
    pub run: AsyncSharedClientTestFunc<T>,
}

/// Like [`NClientTestSpec`], but starts a single client once and runs a list
/// of scenarios against it, without restarting the container between
/// scenarios.
///
/// Useful when a suite has many short read-only scenarios whose per-test
/// container boot dominates wall-clock time (e.g. RPC-compatibility suites
/// that repeatedly query the same finalized state).
///
/// Semantics:
///
/// - A container is started once, owned by a "lifecycle owner" test case
///   created from `name` / `description`. The owner test is always reported
///   as passing; it exists solely to anchor the container's lifecycle.
/// - Each entry in `scenarios` is registered as its own hive test case
///   sharing the owner's container (via
///   [`Simulation::register_multi_test_node`]). Pass/fail/details are
///   recorded per-scenario.
/// - Scenarios run sequentially in the order given. They see each other's
///   side effects on the client (this is the whole point; scenarios that
///   rely on a fresh container must keep using [`NClientTestSpec`]).
/// - `test_data` is cloned once per scenario. Use `Arc`-wrapped payloads to
///   share heap state cheaply.
/// - When all scenarios have ended, the owner test ends, which tears down
///   the container.
#[derive(Clone)]
pub struct SharedClientTestSpec<T> {
    /// Name of the lifecycle-owner test surfaced in hiveview. Pick something
    /// that makes it clear this is a shared-client group (e.g.
    /// `"rpc_compat: shared-client SSZ decode suite"`).
    pub name: String,
    pub description: String,
    /// If true, the owner test runs even when the test matcher would
    /// otherwise filter it out. Per-scenario `always_run` still applies on
    /// top of this.
    pub always_run: bool,
    /// Environment variables for the shared client.
    pub environment: Option<HashMap<String, String>>,
    /// Files to upload before starting the shared client.
    pub files: Option<HashMap<String, Vec<u8>>>,
    /// Test data cloned once per scenario.
    pub test_data: T,
    /// The client to start and share across all scenarios. Only single-client
    /// sharing is supported today; open an issue if multi-client sharing is
    /// needed.
    pub client: ClientDefinition,
    pub scenarios: Vec<SharedClientScenario<T>>,
}

#[async_trait]
impl<T: Clone + Send + Sync + 'static> Testable for SharedClientTestSpec<T> {
    async fn run_test(&self, simulation: Simulation, suite_id: SuiteID, suite: Suite) {
        // Apply the owner-level filter first. If the owner doesn't match and
        // no scenario has an explicit `always_run` override that would match,
        // skip the whole group before we incur container-start cost.
        if let Some(test_match) = simulation.test_matcher.clone() {
            let owner_matches = self.always_run || test_match.match_test(&suite.name, &self.name);
            if !owner_matches {
                let any_scenario_matches = self
                    .scenarios
                    .iter()
                    .any(|s| s.always_run || test_match.match_test(&suite.name, &s.name));
                if !any_scenario_matches {
                    return;
                }
            }
        }

        run_shared_client_test(
            simulation,
            suite_id,
            suite,
            self.name.clone(),
            self.description.clone(),
            self.client.clone(),
            self.environment.clone(),
            self.files.clone(),
            self.test_data.clone(),
            self.scenarios.clone(),
        )
        .await;
    }
}

#[allow(clippy::too_many_arguments)]
async fn run_shared_client_test<T: Clone + Send + Sync + 'static>(
    host: Simulation,
    suite_id: SuiteID,
    suite: Suite,
    owner_name: String,
    owner_desc: String,
    client_def: ClientDefinition,
    environment: Option<HashMap<String, String>>,
    files: Option<HashMap<String, Vec<u8>>>,
    test_data: T,
    scenarios: Vec<SharedClientScenario<T>>,
) {
    // Start the lifecycle owner test first. Its sole role is to own the
    // container; we end it last so the container survives every scenario.
    let owner_test_id = host.start_test(suite_id, owner_name, owner_desc).await;

    let (container_id, ip) = host
        .start_client_with_files(
            suite_id,
            owner_test_id,
            client_def.name.clone(),
            environment,
            files,
        )
        .await;

    let mut scenarios_run: usize = 0;

    for scenario in scenarios {
        if let Some(test_match) = host.test_matcher.clone() {
            if !scenario.always_run && !test_match.match_test(&suite.name, &scenario.name) {
                continue;
            }
        }

        let scenario_test_id = host
            .start_test(
                suite_id,
                scenario.name.clone(),
                scenario.description.clone(),
            )
            .await;

        // Register the owner's container with the scenario's test id. The
        // registered copy has no teardown hook (see `registerMultiTestNode`
        // on the hive server), so ending the scenario leaves the container
        // running for the next one.
        host.register_multi_test_node(suite_id, owner_test_id, &container_id, scenario_test_id)
            .await;

        let scenario_client = Client {
            kind: client_def.name.clone(),
            container: container_id.clone(),
            ip,
            rpc: HttpClientBuilder::default()
                .build(format!("http://{}:8545", ip))
                .expect("Failed to build rpc_client"),
            test: Test {
                sim: host.clone(),
                test_id: scenario_test_id,
                suite: suite.clone(),
                suite_id,
                result: Default::default(),
            },
        };

        let data_clone = test_data.clone();
        let run = scenario.run;
        let test_result = extract_test_results(
            tokio::spawn(async move {
                (run)(scenario_client, data_clone).await;
            })
            .await,
        );

        host.end_test(suite_id, scenario_test_id, test_result).await;
        scenarios_run += 1;
    }

    host.end_test(
        suite_id,
        owner_test_id,
        TestResult {
            pass: true,
            details: format!("shared-client lifecycle owner ({scenarios_run} scenario(s) ran)"),
        },
    )
    .await;
}

pub async fn run_suite(host: Simulation, suites: Vec<Suite>) {
    for suite in suites {
        if let Some(test_match) = host.test_matcher.clone() {
            if !test_match.match_test(&suite.name, "") {
                continue;
            }
        }

        let name = suite.clone().name;
        let description = suite.clone().description;

        let suite_id = host.start_suite(name, description, "".to_string()).await;

        for test in &suite.tests {
            test.run_test(host.clone(), suite_id, suite.clone()).await;
        }

        host.end_suite(suite_id).await;
    }
}
