use crate::types::{ClientDefinition, StartNodeResponse, SuiteID, TestID, TestRequest, TestResult};
use crate::TestMatcher;
use std::collections::HashMap;
use std::env;
use std::net::IpAddr;
use std::str::FromStr;
use std::time::Duration;

/// Timeout for short simulator control-plane requests.
const CONTROL_PLANE_TIMEOUT: Duration = Duration::from_secs(10);

/// Wraps the simulation HTTP API provided by hive.
#[derive(Clone, Debug)]
pub struct Simulation {
    pub url: String,
    pub test_matcher: Option<TestMatcher>,
    /// Shared client for short control-plane requests.
    http_client: reqwest::Client,
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

        let http_client = reqwest::Client::builder()
            .timeout(CONTROL_PLANE_TIMEOUT)
            .build()
            .unwrap_or_else(|_| reqwest::Client::new());

        Self {
            url,
            test_matcher,
            http_client,
        }
    }

    /// Builds a [`Simulation`] for tests against a custom API URL.
    #[doc(hidden)]
    pub fn with_url(url: String) -> Self {
        let http_client = reqwest::Client::builder()
            .timeout(CONTROL_PLANE_TIMEOUT)
            .build()
            .unwrap_or_else(|_| reqwest::Client::new());
        Self {
            url,
            test_matcher: None,
            http_client,
        }
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
        self.start_client_with_files(test_suite, test, client_type, environment, None)
            .await
    }

    /// Starts a new node (or other container), uploading files before startup.
    /// Returns container id and ip.
    pub async fn start_client_with_files(
        &self,
        test_suite: SuiteID,
        test: TestID,
        client_type: String,
        environment: Option<HashMap<String, String>>,
        files: Option<HashMap<String, Vec<u8>>>,
    ) -> (String, IpAddr) {
        let url = format!("{}/testsuite/{}/test/{}/node", self.url, test_suite, test);
        let client = reqwest::Client::new();

        let mut config = SimulatorConfig::new();
        config.client = client_type;
        if let Some(environment) = environment {
            config.environment = environment;
        }

        let config = serde_json::to_string(&config).expect("Failed to parse config to serde_json");
        let mut form = reqwest::multipart::Form::new().text("config", config);
        if let Some(files) = files {
            for (path, contents) in files {
                form = form.part(
                    path,
                    reqwest::multipart::Part::bytes(contents).file_name("hive-upload"),
                );
            }
        }

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

    /// Registers an existing container with another test without starting a new one.
    pub async fn register_multi_test_node(
        &self,
        test_suite: SuiteID,
        source_test: TestID,
        node_id: &str,
        target_test: TestID,
    ) -> Result<(), String> {
        let url = format!(
            "{}/testsuite/{}/test/{}/node/{}/register/{}",
            self.url, test_suite, source_test, node_id, target_test
        );
        let resp = self.http_client.post(&url).send().await.map_err(|err| {
            format!(
                "register_multi_test_node transport error (node={node_id}, \
                 source_test={source_test}, target_test={target_test}): {err}"
            )
        })?;

        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(format!(
                "register_multi_test_node rejected (node={node_id}, \
                 source_test={source_test}, target_test={target_test}): \
                 status={status} body={body}"
            ));
        }
        Ok(())
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

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::TcpListener;
    use std::sync::atomic::{AtomicUsize, Ordering};
    use std::sync::Arc;
    use tokio::io::{AsyncReadExt, AsyncWriteExt};
    use tokio::net::TcpListener as TokioTcpListener;

    /// Minimal one-request HTTP test server.
    async fn spawn_mock(
        response: &'static str,
    ) -> (String, Arc<AtomicUsize>, tokio::task::JoinHandle<()>) {
        let std_listener = TcpListener::bind("127.0.0.1:0").expect("bind mock");
        std_listener.set_nonblocking(true).expect("nonblocking");
        let addr = std_listener.local_addr().expect("addr");
        let listener = TokioTcpListener::from_std(std_listener).expect("tokio listener");
        let hits = Arc::new(AtomicUsize::new(0));
        let hits_clone = hits.clone();
        let handle = tokio::spawn(async move {
            loop {
                let (mut socket, _) = match listener.accept().await {
                    Ok(v) => v,
                    Err(_) => break,
                };
                hits_clone.fetch_add(1, Ordering::SeqCst);
                let mut buf = [0u8; 1024];
                let _ = socket.read(&mut buf).await;
                let _ = socket.write_all(response.as_bytes()).await;
                let _ = socket.shutdown().await;
            }
        });
        (format!("http://{}", addr), hits, handle)
    }

    #[tokio::test]
    async fn register_multi_test_node_happy_path() {
        let (url, hits, handle) =
            spawn_mock("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK").await;
        let sim = Simulation::with_url(url);
        let result = sim.register_multi_test_node(1, 2, "node-abc", 3).await;
        handle.abort();
        assert!(result.is_ok(), "expected Ok, got {result:?}");
        assert_eq!(hits.load(Ordering::SeqCst), 1);
    }

    #[tokio::test]
    async fn register_multi_test_node_returns_err_on_non_2xx() {
        let (url, _, handle) = spawn_mock(
            "HTTP/1.1 500 Internal Server Error\r\nContent-Length: 11\r\n\
             Connection: close\r\n\r\nserver boom",
        )
        .await;
        let sim = Simulation::with_url(url);
        let result = sim.register_multi_test_node(1, 2, "node-abc", 3).await;
        handle.abort();
        let err = result.expect_err("expected Err on 500");
        assert!(err.contains("status=500"), "unexpected err: {err}");
        assert!(err.contains("server boom"), "unexpected err: {err}");
    }

    #[tokio::test]
    async fn register_multi_test_node_returns_err_on_transport_failure() {
        let addr = {
            let l = TcpListener::bind("127.0.0.1:0").expect("bind");
            l.local_addr().expect("addr")
        };
        let sim = Simulation::with_url(format!("http://{addr}"));
        let result = sim.register_multi_test_node(1, 2, "node-abc", 3).await;
        let err = result.expect_err("expected Err on closed port");
        assert!(err.contains("transport error"), "unexpected err: {err}");
    }
}
