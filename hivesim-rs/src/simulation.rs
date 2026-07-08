use crate::types::{
    ApiError, ClientDefinition, StartNodeResponse, SuiteID, TestID, TestRequest, TestResult,
};
use crate::TestMatcher;
use std::collections::HashMap;
use std::env;
use std::net::IpAddr;
use std::str::FromStr;
use std::sync::{Arc, Mutex};
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
    planned_tests: Option<Arc<Mutex<Vec<String>>>>,
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
            planned_tests: None,
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
            planned_tests: None,
        }
    }

    pub(crate) fn planning_clone(&self) -> Self {
        Self {
            url: self.url.clone(),
            test_matcher: self.test_matcher.clone(),
            http_client: self.http_client.clone(),
            planned_tests: Some(Arc::new(Mutex::new(Vec::new()))),
        }
    }

    pub(crate) fn is_planning(&self) -> bool {
        self.planned_tests.is_some()
    }

    pub(crate) fn record_planned_test(&self, suite: &str, test: &str, always_run: bool) {
        let Some(planned_tests) = &self.planned_tests else {
            return;
        };
        if let Some(test_matcher) = &self.test_matcher {
            if !always_run && !test_matcher.match_test(suite, test) {
                return;
            }
        }
        planned_tests
            .lock()
            .expect("planned test collector lock poisoned")
            .push(test.to_string());
    }

    pub(crate) fn planned_tests(&self) -> Vec<String> {
        self.planned_tests
            .as_ref()
            .map(|planned_tests| {
                planned_tests
                    .lock()
                    .expect("planned test collector lock poisoned")
                    .clone()
            })
            .unwrap_or_default()
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

    pub fn test_progress(&self, suite: &str) {
        println!(
            "HIVE_SUITE_PROGRESS {}",
            serde_json::json!({ "suite": suite })
        );
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

        let response = client
            .post(url)
            .multipart(form)
            .send()
            .await
            .expect("Failed to send start client request");
        let status = response.status();
        let body = response
            .bytes()
            .await
            .expect("Failed to read start client response body");

        // Surface hive's real error (e.g. "client did not start: ...") rather
        // than the opaque serde "missing field `id`" that results from blindly
        // decoding an error body as a StartNodeResponse.
        let resp = parse_start_node_response(status, &body)
            .unwrap_or_else(|err| panic!("Failed to start client: {err}"));

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

    /// Stops a running client container.
    pub async fn stop_client(
        &self,
        test_suite: SuiteID,
        test: TestID,
        node_id: &str,
    ) -> Result<(), String> {
        let url = format!(
            "{}/testsuite/{}/test/{}/node/{}",
            self.url, test_suite, test, node_id
        );
        let resp = self.http_client.delete(&url).send().await.map_err(|err| {
            format!("stop_client transport error (node={node_id}, test={test}): {err}")
        })?;

        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(format!(
                "stop_client rejected (node={node_id}, test={test}): status={status} body={body}"
            ));
        }

        Ok(())
    }

    /// Pauses a running client container.
    pub async fn pause_client(
        &self,
        test_suite: SuiteID,
        test: TestID,
        node_id: &str,
    ) -> Result<(), String> {
        let url = format!(
            "{}/testsuite/{}/test/{}/node/{}/pause",
            self.url, test_suite, test, node_id
        );
        let resp = self.http_client.post(&url).send().await.map_err(|err| {
            format!("pause_client transport error (node={node_id}, test={test}): {err}")
        })?;

        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(format!(
                "pause_client rejected (node={node_id}, test={test}): status={status} body={body}"
            ));
        }

        Ok(())
    }

    /// Unpauses a paused client container.
    pub async fn unpause_client(
        &self,
        test_suite: SuiteID,
        test: TestID,
        node_id: &str,
    ) -> Result<(), String> {
        let url = format!(
            "{}/testsuite/{}/test/{}/node/{}/pause",
            self.url, test_suite, test, node_id
        );
        let resp = self.http_client.delete(&url).send().await.map_err(|err| {
            format!("unpause_client transport error (node={node_id}, test={test}): {err}")
        })?;

        let status = resp.status();
        if !status.is_success() {
            let body = resp.text().await.unwrap_or_default();
            return Err(format!(
                "unpause_client rejected (node={node_id}, test={test}): status={status} body={body}"
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

/// Interpret hive's response to a start-client request.
///
/// On success hive returns HTTP 200 with a [`StartNodeResponse`]
/// (`{"id","ip"}`). On failure it returns a non-2xx status with an
/// [`ApiError`] body (`{"error": "..."}`) whose message is the actual cause,
/// e.g. `client did not start: ...` when the container exits during startup
/// (see `startClient`/`serveError` in hive's `internal/libhive/api.go`).
///
/// Decoding every response as a [`StartNodeResponse`] regardless of status
/// turns that real failure into an opaque serde `missing field \`id\`` error,
/// so extract hive's own message (falling back to the raw body) instead.
fn parse_start_node_response(
    status: reqwest::StatusCode,
    body: &[u8],
) -> Result<StartNodeResponse, String> {
    if status.is_success() {
        if let Ok(resp) = serde_json::from_slice::<StartNodeResponse>(body) {
            return Ok(resp);
        }
    }

    let detail = serde_json::from_slice::<ApiError>(body)
        .map(|api_err| api_err.error)
        .unwrap_or_else(|_| String::from_utf8_lossy(body).trim().to_string());

    Err(format!(
        "hive rejected start-client request (HTTP {}): {detail}",
        status.as_u16()
    ))
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
        (format!("http://{addr}"), hits, handle)
    }

    #[test]
    fn parse_start_node_response_success() {
        let body = br#"{"id":"abc123","ip":"172.17.0.4"}"#;
        let resp = parse_start_node_response(reqwest::StatusCode::OK, body)
            .expect("valid StartNodeResponse should parse");
        assert_eq!(resp.id, "abc123");
        assert_eq!(resp.ip, "172.17.0.4");
    }

    #[test]
    fn parse_start_node_response_surfaces_backend_error() {
        // Mirrors hive's serveError body when the container exits during startup.
        let body = br#"{"error":"client did not start: container exited"}"#;
        let err = parse_start_node_response(reqwest::StatusCode::INTERNAL_SERVER_ERROR, body)
            .expect_err("an error body must not be treated as success");
        assert!(
            err.contains("client did not start: container exited"),
            "should surface hive's message, got: {err}"
        );
        assert!(err.contains("500"), "should include the status, got: {err}");
        // Regression guard: the old code masked this as serde's "missing field `id`".
        assert!(
            !err.contains("missing field"),
            "must not leak the serde decode error, got: {err}"
        );
    }

    #[test]
    fn parse_start_node_response_falls_back_to_raw_body() {
        // A non-JSON error body (e.g. from a proxy) is surfaced verbatim.
        let body = b"502 upstream unavailable";
        let err = parse_start_node_response(reqwest::StatusCode::BAD_GATEWAY, body)
            .expect_err("non-success status must be an error");
        assert!(
            err.contains("502 upstream unavailable"),
            "should include the raw body, got: {err}"
        );
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
