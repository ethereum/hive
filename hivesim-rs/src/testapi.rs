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

/// One scenario in a [`SharedClientTestSpec`].
#[derive(Clone)]
pub struct SharedClientScenario<T> {
    pub name: String,
    pub description: String,
    /// Runs even if filtered by the test matcher.
    pub always_run: bool,
    pub run: AsyncSharedClientTestFunc<T>,
}

/// Runs multiple scenarios against one client container.
#[derive(Clone)]
pub struct SharedClientTestSpec<T> {
    /// Name of the lifecycle-owner test shown in hiveview.
    pub name: String,
    pub description: String,
    /// Runs even if filtered by the test matcher.
    pub always_run: bool,
    pub environment: Option<HashMap<String, String>>,
    pub files: Option<HashMap<String, Vec<u8>>>,
    pub test_data: T,
    /// Client shared across all scenarios.
    pub client: ClientDefinition,
    pub scenarios: Vec<SharedClientScenario<T>>,
}

#[async_trait]
impl<T: Clone + Send + Sync + 'static> Testable for SharedClientTestSpec<T> {
    async fn run_test(&self, simulation: Simulation, suite_id: SuiteID, suite: Suite) {
        // Skip the shared client entirely if neither the owner nor any scenario matches.
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

/// Best-effort teardown guard for the shared-client owner test.
struct OwnerGuard {
    host: Simulation,
    suite_id: SuiteID,
    owner_test_id: TestID,
    disarmed: bool,
}

impl OwnerGuard {
    fn new(host: Simulation, suite_id: SuiteID, owner_test_id: TestID) -> Self {
        Self {
            host,
            suite_id,
            owner_test_id,
            disarmed: true,
        }
    }

    fn arm(&mut self) {
        self.disarmed = false;
    }

    fn disarm(&mut self) {
        self.disarmed = true;
    }
}

impl Drop for OwnerGuard {
    fn drop(&mut self) {
        if self.disarmed {
            return;
        }
        let host = self.host.clone();
        let suite_id = self.suite_id;
        let owner_test_id = self.owner_test_id;
        // Best-effort cleanup if the scenario loop aborts early.
        if let Ok(handle) = tokio::runtime::Handle::try_current() {
            handle.spawn(async move {
                host.end_test(
                    suite_id,
                    owner_test_id,
                    TestResult {
                        pass: false,
                        details: "shared-client owner aborted before completion".to_string(),
                    },
                )
                .await;
            });
        }
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
    // The owner test keeps the shared container alive across scenarios.
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

    let mut guard = OwnerGuard::new(host.clone(), suite_id, owner_test_id);
    guard.arm();

    let mut scenarios_run: usize = 0;
    let mut scenarios_passed: usize = 0;
    let mut scenarios_filtered: usize = 0;

    for scenario in scenarios {
        if let Some(test_match) = host.test_matcher.clone() {
            if !scenario.always_run && !test_match.match_test(&suite.name, &scenario.name) {
                scenarios_filtered += 1;
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

        // Registration failure fails only this scenario.
        if let Err(err) = host
            .register_multi_test_node(suite_id, owner_test_id, &container_id, scenario_test_id)
            .await
        {
            host.end_test(
                suite_id,
                scenario_test_id,
                TestResult {
                    pass: false,
                    details: format!("shared-client registration failed; skipping scenario: {err}"),
                },
            )
            .await;
            scenarios_run += 1;
            continue;
        }

        let data_clone = test_data.clone();
        let run = scenario.run;
        let host_for_spawn = host.clone();
        let kind_for_spawn = client_def.name.clone();
        let container_for_spawn = container_id.clone();
        let suite_for_spawn = suite.clone();
        let test_result = extract_test_results(
            tokio::spawn(async move {
                let scenario_client = Client {
                    kind: kind_for_spawn,
                    container: container_for_spawn,
                    ip,
                    rpc: HttpClientBuilder::default()
                        .build(format!("http://{}:8545", ip))
                        .expect("Failed to build rpc_client"),
                    test: Test {
                        sim: host_for_spawn,
                        test_id: scenario_test_id,
                        suite: suite_for_spawn,
                        suite_id,
                        result: Default::default(),
                    },
                };
                (run)(scenario_client, data_clone).await;
            })
            .await,
        );

        if test_result.pass {
            scenarios_passed += 1;
        }
        host.end_test(suite_id, scenario_test_id, test_result).await;
        scenarios_run += 1;
    }

    let owner_pass = scenarios_run > 0 && scenarios_passed == scenarios_run;
    let owner_details = format!(
        "shared-client lifecycle owner: {scenarios_passed}/{scenarios_run} \
         scenario(s) passed ({scenarios_filtered} filtered)"
    );
    host.end_test(
        suite_id,
        owner_test_id,
        TestResult {
            pass: owner_pass,
            details: owner_details,
        },
    )
    .await;
    guard.disarm();
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
