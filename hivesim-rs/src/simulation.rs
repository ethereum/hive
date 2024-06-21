use crate::types::{ClientDefinition, StartNodeResponse, SuiteID, TestID, TestRequest, TestResult};
use crate::TestMatcher;
use std::collections::HashMap;
use std::env;
use std::net::IpAddr;
use std::str::FromStr;

/// Wraps the simulation HTTP API provided by hive.
#[derive(Clone, Debug)]
pub struct Simulation {
    pub url: String,
    pub test_matcher: Option<TestMatcher>,
}

impl Default for Simulation {
    fn default() -> Self {
        Self::new()
    }
}

// A struct in the structure of the JSON config shown in simulators.md
// it is used to pass information to the Hive Simulators
#[derive(serde::Serialize, serde::Deserialize)]
struct SimulatorConfig {
    client: String,
    environment: HashMap<String, String>,
}

impl SimulatorConfig {
    pub fn new() -> Self {
        Self {
            client: "".to_string(),
            environment: Default::default(),
        }
    }
}

impl Simulation {
    /// New looks up the hive host URI using the HIVE_SIMULATOR environment variable
    /// and connects to it. It will panic if HIVE_SIMULATOR is not set.
    pub fn new() -> Self {
        let url = env::var("HIVE_SIMULATOR").expect("HIVE_SIMULATOR environment variable not set");
        let test_matcher = match env::var("HIVE_TEST_PATTERN") {
            Ok(pattern) => {
                if pattern.is_empty() {
                    None
                } else {
                    Some(TestMatcher::new(&pattern))
                }
            }
            Err(_) => None,
        };

        if url.is_empty() {
            panic!("HIVE_SIMULATOR environment variable is empty")
        }

        Self { url, test_matcher }
    }

    pub async fn start_suite(
        &self,
        name: String,
        description: String,
        _sim_log: String,
    ) -> SuiteID {
        let url = format!("{}/testsuite", self.url);
        let client = reqwest::Client::new();
        let body = TestRequest { name, description };

        client
            .post(url)
            .json(&body)
            .send()
            .await
            .expect("Failed to send start suite request")
            .json::<SuiteID>()
            .await
            .expect("Failed to convert start suite response to json")
    }

    pub async fn end_suite(&self, test_suite: SuiteID) {
        let url = format!("{}/testsuite/{}", self.url, test_suite);
        let client = reqwest::Client::new();
        client
            .delete(url)
            .send()
            .await
            .expect("Failed to send an end suite request");
    }

    /// Starts a new test case, returning the testcase id as a context identifier
    pub async fn start_test(
        &self,
        test_suite: SuiteID,
        name: String,
        description: String,
    ) -> TestID {
        let url = format!("{}/testsuite/{}/test", self.url, test_suite);
        let client = reqwest::Client::new();
        let body = TestRequest { name, description };

        client
            .post(url)
            .json(&body)
            .send()
            .await
            .expect("Failed to send start test request")
            .json::<TestID>()
            .await
            .expect("Failed to convert start test response to json")
    }

    /// Finishes the test case, cleaning up everything, logging results, and returning
    /// an error if the process could not be completed.
    pub async fn end_test(&self, test_suite: SuiteID, test: TestID, test_result: TestResult) {
        let url = format!("{}/testsuite/{}/test/{}", self.url, test_suite, test);
        let client = reqwest::Client::new();

        client
            .post(url)
            .json(&test_result)
            .send()
            .await
            .expect("Failed to send end test request");
    }

    /// Starts a new node (or other container).
    /// Returns container id and ip.
    pub async fn start_client(
        &self,
        test_suite: SuiteID,
        test: TestID,
        client_type: String,
        environment: Option<HashMap<String, String>>,
    ) -> (String, IpAddr) {
        let url = format!("{}/testsuite/{}/test/{}/node", self.url, test_suite, test);
        let client = reqwest::Client::new();

        let mut config = SimulatorConfig::new();
        config.client = client_type;
        if let Some(environment) = environment {
            config.environment = environment;
        }

        let config = serde_json::to_string(&config).expect("Failed to parse config to serde_json");
        let form = reqwest::multipart::Form::new().text("config", config);

        let resp = client
            .post(url)
            .multipart(form)
            .send()
            .await
            .expect("Failed to send start client request")
            .json::<StartNodeResponse>()
            .await
            .expect("Failed to convert start node response to json");

        let ip = IpAddr::from_str(&resp.ip).expect("Failed to decode IpAddr from string");

        (resp.id, ip)
    }

    /// Returns all client types available to this simulator run. This depends on
    /// both the available client set and the command line filters.
    pub async fn client_types(&self) -> Vec<ClientDefinition> {
        let url = format!("{}/clients", self.url);
        let client = reqwest::Client::new();
        client
            .get(&url)
            .send()
            .await
            .expect("Failed to send get client types request")
            .json::<Vec<ClientDefinition>>()
            .await
            .expect("Failed to convert client types response to json")
    }
}
