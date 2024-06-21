use crate::types::{ClientDefinition, SuiteID, TestData, TestID, TestResult};
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

pub type AsyncNClientsTestFunc = fn(
    Vec<Client>,
    Option<TestData>,
) -> Pin<
    Box<
        dyn Future<Output = ()> // future API / pollable
            + Send // required by non-single-threaded executors
            + 'static,
    >,
>;

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
        let (container, ip) = self
            .sim
            .start_client(
                self.suite_id,
                self.test_id,
                client_type.clone(),
                environment,
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
pub struct NClientTestSpec {
    /// These fields are displayed in the UI. Be sure to add
    /// a meaningful description here.
    pub name: String,
    pub description: String,
    /// If AlwaysRun is true, the test will run even if Name does not match the test
    /// pattern. This option is useful for tests that launch a client instance and
    /// then perform further tests against it.
    pub always_run: bool,
    /// The Run function is invoked when the test executes.
    pub run: AsyncNClientsTestFunc,
    /// For each client, there is a distinct map of Hive Environment Variable names to values.
    /// The environments must be in the same order as the `clients`
    pub environments: Option<Vec<Option<HashMap<String, String>>>>,
    /// test data which can be passed to the test
    pub test_data: Option<TestData>,
    pub clients: Vec<ClientDefinition>,
}

#[async_trait]
impl Testable for NClientTestSpec {
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
            self.test_data.to_owned(),
            self.clients.to_owned(),
            self.run,
        )
        .await;
    }
}

// Write a test that runs against N clients.
async fn run_n_client_test(
    host: Simulation,
    test: TestRun,
    environments: Option<Vec<Option<HashMap<String, String>>>>,
    test_data: Option<TestData>,
    clients: Vec<ClientDefinition>,
    func: AsyncNClientsTestFunc,
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
            for (client, environment) in clients.into_iter().zip(env_iter) {
                client_vec.push(test.start_client(client.name.to_owned(), environment).await);
            }
            (func)(client_vec, test_data).await;
        })
        .await,
    );

    host.end_test(suite_id, test_id, test_result).await;
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
