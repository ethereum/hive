use crate::types::{ClientDefinition, SuiteID, TestID, TestResult};
use crate::{Simulation, TestMatcher};
use ::std::{boxed::Box, future::Future, pin::Pin};
use async_trait::async_trait;
use core::fmt::Debug;
use dyn_clone::DynClone;
use jsonrpsee::http_client::{HttpClient, HttpClientBuilder};
use std::collections::HashMap;
use std::io::Write;
use std::net::IpAddr;
use std::time::Duration;
use tokio::time::timeout;

use crate::utils::extract_test_results;

const SHARED_CLIENT_STARTUP_TIMEOUT: Duration = Duration::from_secs(120);

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

    async fn planned_test_count(&self, simulation: &Simulation, suite: &Suite) -> usize {
        self.planned_test_names(&suite.name, simulation.test_matcher.as_ref())
            .len()
    }

    fn planned_test_names(&self, suite: &str, test_matcher: Option<&TestMatcher>) -> Vec<String> {
        let _ = (suite, test_matcher);
        Vec::new()
    }
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
    pub count_progress: bool,
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

        let rpc_url = format!("http://{ip}:8545");

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
        if self.sim.is_planning() {
            for name in spec.planned_test_names(&self.suite.name, self.sim.test_matcher.as_ref()) {
                self.sim.record_planned_test(&self.suite.name, &name, true);
            }
            return;
        }
        spec.run_test(self.sim.clone(), self.suite_id, self.suite.clone())
            .await
    }

    pub fn plan_test(&self, name: &str, always_run: bool) -> bool {
        if !self.sim.is_planning() {
            return false;
        }
        self.sim
            .record_planned_test(&self.suite.name, name, always_run);
        true
    }
}

impl Client {
    /// Stops the client container.
    pub async fn stop(&self) -> Result<(), String> {
        self.test
            .sim
            .stop_client(self.test.suite_id, self.test.test_id, &self.container)
            .await
    }

    /// Pauses the client container.
    pub async fn pause(&self) -> Result<(), String> {
        self.test
            .sim
            .pause_client(self.test.suite_id, self.test.test_id, &self.container)
            .await
    }

    /// Unpauses the client container.
    pub async fn unpause(&self) -> Result<(), String> {
        self.test
            .sim
            .unpause_client(self.test.suite_id, self.test.test_id, &self.container)
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
            count_progress: true,
        };

        run_test(simulation, test_run, self.client.clone(), self.run).await;
    }

    fn planned_test_names(&self, suite: &str, test_matcher: Option<&TestMatcher>) -> Vec<String> {
        if planned_test_count_for_name(suite, &self.name, self.always_run, test_matcher) == 0 {
            Vec::new()
        } else {
            vec![self.name.clone()]
        }
    }
}

#[derive(Clone)]
pub struct PlannedTestSpec {
    pub name: String,
    pub description: String,
    pub always_run: bool,
    pub run: AsyncTestFunc,
    pub client: Option<Client>,
}

#[async_trait]
impl Testable for PlannedTestSpec {
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
            count_progress: false,
        };

        run_test(simulation, test_run, self.client.clone(), self.run).await;
    }

    async fn planned_test_count(&self, simulation: &Simulation, suite: &Suite) -> usize {
        if let Some(test_match) = simulation.test_matcher.clone() {
            if !self.always_run && !test_match.match_test(&suite.name, &self.name) {
                return 0;
            }
        }

        let planning_simulation = simulation.planning_clone();
        let mut test = Test {
            sim: planning_simulation.clone(),
            test_id: 0,
            suite: suite.clone(),
            suite_id: 0,
            result: Default::default(),
        };
        (self.run)(&mut test, self.client.clone()).await;
        planning_simulation.planned_tests().len()
    }
}

pub async fn run_test(
    host: Simulation,
    test: TestRun,
    client: Option<Client>,
    func: AsyncTestFunc,
) {
    // Register test on simulation server and initialize the T.
    let suite_name = test.suite.name.clone();
    let count_progress = test.count_progress;
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
    if count_progress {
        host.test_progress(&suite_name);
    }
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
            count_progress: true,
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

    fn planned_test_names(&self, suite: &str, test_matcher: Option<&TestMatcher>) -> Vec<String> {
        if planned_test_count_for_name(suite, &self.name, self.always_run, test_matcher) == 0 {
            Vec::new()
        } else {
            vec![self.name.clone()]
        }
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
    let suite_name = test.suite.name.clone();
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
    host.test_progress(&suite_name);
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

    fn planned_test_names(&self, suite: &str, test_matcher: Option<&TestMatcher>) -> Vec<String> {
        let scenario_names = self
            .scenarios
            .iter()
            .filter_map(|scenario| {
                if planned_test_count_for_name(
                    suite,
                    &scenario.name,
                    scenario.always_run,
                    test_matcher,
                ) == 0
                {
                    None
                } else {
                    Some(scenario.name.clone())
                }
            })
            .collect::<Vec<_>>();
        let include_owner = match test_matcher {
            Some(test_matcher) => {
                let owner_matches = self.always_run || test_matcher.match_test(suite, &self.name);
                owner_matches || !scenario_names.is_empty()
            }
            None => true,
        };
        if include_owner {
            let mut names = Vec::with_capacity(scenario_names.len() + 1);
            names.push(self.name.clone());
            names.extend(scenario_names);
            names
        } else {
            scenario_names
        }
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

fn join_error_to_string(error: tokio::task::JoinError) -> String {
    if error.is_panic() {
        let payload = error.into_panic();
        if let Some(error) = payload.downcast_ref::<&'static str>() {
            error.to_string()
        } else if let Some(error) = payload.downcast_ref::<String>() {
            error.clone()
        } else {
            format!("unknown panic payload: {payload:?}")
        }
    } else {
        error.to_string()
    }
}

async fn start_shared_client(
    host: &Simulation,
    suite_id: SuiteID,
    owner_test_id: TestID,
    client_name: &str,
    environment: Option<HashMap<String, String>>,
    files: Option<HashMap<String, Vec<u8>>>,
) -> Result<(String, IpAddr), String> {
    let host = host.clone();
    let client_name_for_start = client_name.to_string();
    let mut handle = tokio::spawn(async move {
        host.start_client_with_files(
            suite_id,
            owner_test_id,
            client_name_for_start,
            environment,
            files,
        )
        .await
    });

    match timeout(SHARED_CLIENT_STARTUP_TIMEOUT, &mut handle).await {
        Ok(Ok(client)) => Ok(client),
        Ok(Err(error)) => Err(format!(
            "startup task failed: {}",
            join_error_to_string(error)
        )),
        Err(_) => {
            handle.abort();
            let _ = handle.await;
            Err(format!(
                "startup exceeded {} seconds",
                SHARED_CLIENT_STARTUP_TIMEOUT.as_secs()
            ))
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
    let mut guard = OwnerGuard::new(host.clone(), suite_id, owner_test_id);
    guard.arm();

    let (container_id, ip) = match start_shared_client(
        &host,
        suite_id,
        owner_test_id,
        &client_def.name,
        environment,
        files,
    )
    .await
    {
        Ok(client) => client,
        Err(err) => {
            let mut scenarios_failed = 0;
            let mut scenarios_filtered = 0;

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
                host.end_test(
                    suite_id,
                    scenario_test_id,
                    TestResult {
                        pass: false,
                        details: format!(
                            "shared-client startup failed for {}; skipping scenario: {err}",
                            client_def.name
                        ),
                    },
                )
                .await;
                host.test_progress(&suite.name);
                scenarios_failed += 1;
            }

            host.end_test(
                suite_id,
                owner_test_id,
                TestResult {
                    pass: false,
                    details: format!(
                        "shared-client startup failed for {}; {scenarios_failed} scenario(s) failed ({scenarios_filtered} filtered): {err}",
                        client_def.name
                    ),
                },
            )
            .await;
            host.test_progress(&suite.name);
            guard.disarm();
            return;
        }
    };

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
            host.test_progress(&suite.name);
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
                        .build(format!("http://{ip}:8545"))
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
        host.test_progress(&suite.name);
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
    host.test_progress(&suite.name);
    guard.disarm();
}

pub async fn run_suite(host: Simulation, suites: Vec<Suite>) {
    run_suite_with_plan_metadata(host, suites, serde_json::json!({})).await;
}

pub async fn run_suite_with_plan_metadata(
    host: Simulation,
    suites: Vec<Suite>,
    plan_metadata: serde_json::Value,
) {
    let suites = suites
        .into_iter()
        .filter(|suite| {
            if let Some(test_match) = host.test_matcher.clone() {
                return test_match.match_test(&suite.name, "");
            }
            true
        })
        .collect::<Vec<_>>();
    let mut suite_plan = Vec::with_capacity(suites.len());
    for suite in &suites {
        let mut tests = 0;
        for test in &suite.tests {
            tests += test.planned_test_count(&host, suite).await;
        }
        suite_plan.push(serde_json::json!({ "name": suite.name.as_str(), "tests": tests }));
    }
    let mut plan = serde_json::json!({ "suites": suite_plan });
    if let (Some(plan), Some(metadata)) = (plan.as_object_mut(), plan_metadata.as_object()) {
        for (key, value) in metadata {
            plan.insert(key.clone(), value.clone());
        }
    }
    println!("HIVE_SUITE_PLAN {}", plan);
    let heartbeat = tokio::spawn(async {
        loop {
            println!("HIVE_RUN_HEARTBEAT {}", serde_json::json!({}));
            tokio::time::sleep(Duration::from_secs(10)).await;
        }
    });

    let run = async move {
        for suite in suites {
            let name = suite.clone().name;
            let description = suite.clone().description;

            let suite_id = host.start_suite(name, description, "".to_string()).await;

            for test in &suite.tests {
                test.run_test(host.clone(), suite_id, suite.clone()).await;
            }

            host.end_suite(suite_id).await;
        }
    };

    tokio::select! {
        _ = run => {
            heartbeat.abort();
            println!("HIVE_RUN_COMPLETE {}", serde_json::json!({}));
        }
        interrupt = run_interrupt_signal() => {
            heartbeat.abort();
            println!(
                "HIVE_RUN_INTERRUPTED {}",
                serde_json::json!({ "signal": interrupt.signal })
            );
            let _ = std::io::stdout().flush();
            std::process::exit(interrupt.exit_code);
        }
    };
}

struct RunInterrupt {
    signal: &'static str,
    exit_code: i32,
}

#[cfg(unix)]
async fn run_interrupt_signal() -> RunInterrupt {
    use tokio::signal::unix::{signal, SignalKind};

    let mut interrupt = signal(SignalKind::interrupt()).expect("failed to register SIGINT handler");
    let mut terminate =
        signal(SignalKind::terminate()).expect("failed to register SIGTERM handler");

    tokio::select! {
        _ = interrupt.recv() => RunInterrupt { signal: "SIGINT", exit_code: 130 },
        _ = terminate.recv() => RunInterrupt { signal: "SIGTERM", exit_code: 143 },
    }
}

#[cfg(not(unix))]
async fn run_interrupt_signal() -> RunInterrupt {
    let _ = tokio::signal::ctrl_c().await;
    RunInterrupt {
        signal: "interrupt",
        exit_code: 130,
    }
}

fn planned_test_count_for_name(
    suite: &str,
    test: &str,
    always_run: bool,
    test_matcher: Option<&TestMatcher>,
) -> usize {
    if let Some(test_matcher) = test_matcher {
        if !always_run && !test_matcher.match_test(suite, test) {
            return 0;
        }
    }
    1
}
