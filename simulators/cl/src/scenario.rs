//! The `cl` simulator has one PlannedTestSpec ("spec-tests"). Its run
//! function:
//!   1. queries hive for all selected clients with role `cl-spec-runner`
//!   2. dynamically spawns one hive test per client via
//!      `Simulation::start_test` / `end_test`
//!   3. for each: starts the client container, polls /status, fetches
//!      cl-meta.json + JUnit XMLs over HTTP, aggregates pass/fail
//!
//! Dynamic test spawning (rather than statically registering per-client
//! Testable instances) is the only path here: hivesim-rs does not export
//! the Testable trait publicly, and the standard `PlannedTestSpec.run`
//! field is a `fn` pointer that cannot carry per-client state.

use std::collections::HashMap;
use std::env;
use std::fmt::Write as _;
use std::future::Future;
use std::net::IpAddr;
use std::pin::Pin;
use std::time::{Duration, Instant};

use hivesim::types::{SuiteID, TestID, TestResult};
use hivesim::{Client, Test};

use crate::junit;
use crate::meta::RunMeta;

const STATUS_PORT: u16 = 5151;
const POLL_INTERVAL: Duration = Duration::from_secs(20);
const ABSOLUTE_TIMEOUT: Duration = Duration::from_secs(2 * 60 * 60);
const HTTP_TIMEOUT: Duration = Duration::from_secs(30);
const MAX_FAILED_CASES_IN_DETAILS: usize = 50;

/// Hive client role that flags an image as a consensus-spec-tests runner.
pub const CL_SPEC_ROLE: &str = "cl-spec-runner";

/// PlannedTestSpec.run entrypoint. Must be a `fn` pointer.
pub fn run_spec_tests(
    test: &mut Test,
    _client: Option<Client>,
) -> Pin<Box<dyn Future<Output = ()> + Send + '_>> {
    Box::pin(async move {
        // Discover spec-runner clients selected for this run.
        let mut spec_runners: Vec<_> = test
            .sim
            .client_types()
            .await
            .into_iter()
            .filter(|c| c.meta.roles.iter().any(|r| r == CL_SPEC_ROLE))
            .collect();
        spec_runners.sort_by(|a, b| a.name.cmp(&b.name));

        // Planning pass: `plan_test()` returns true when hive is counting
        // tests ahead of the real run; record the per-client subtest names
        // and stop.
        let mut is_planning = false;
        for client in &spec_runners {
            if test.plan_test(&client.name, true) {
                is_planning = true;
            }
        }
        if is_planning {
            return;
        }

        if spec_runners.is_empty() {
            tracing::warn!("no spec-runner clients selected; nothing to do");
            return;
        }

        for client_def in spec_runners {
            run_one_subtest(test, &client_def.name).await;
        }
    })
}

async fn run_one_subtest(test: &Test, client_name: &str) {
    let description = format!(
        "Builds {client_name} from source, runs consensus-spec-tests, \
         reports aggregated pass/fail per JUnit suite."
    );

    let subtest_id = test
        .sim
        .start_test(test.suite_id, client_name.to_string(), description)
        .await;

    let result = run_one_client(&test.sim, test.suite_id, subtest_id, client_name).await;

    test.sim.end_test(test.suite_id, subtest_id, result).await;
    test.sim.test_progress(&test.suite.name);
}

async fn run_one_client(
    sim: &hivesim::Simulation,
    suite_id: SuiteID,
    test_id: TestID,
    client_name: &str,
) -> TestResult {
    let env = build_env_for_client();
    let (container_id, ip) = sim
        .start_client_with_files(suite_id, test_id, client_name.to_string(), Some(env), None)
        .await;

    tracing::info!(
        client = client_name,
        ip = %ip,
        container = container_id.as_str(),
        "started spec-runner container",
    );

    let outcome = harvest(client_name, ip).await;

    if let Err(err) = sim.stop_client(suite_id, test_id, &container_id).await {
        tracing::warn!(
            client = client_name,
            container = container_id.as_str(),
            error = %err,
            "stop_client failed",
        );
    }

    outcome
}

async fn harvest(client_name: &str, ip: IpAddr) -> TestResult {
    let http = match reqwest::Client::builder().timeout(HTTP_TIMEOUT).build() {
        Ok(c) => c,
        Err(err) => return fail(format!("failed to build http client: {err}")),
    };

    let base = format!("http://{ip}:{STATUS_PORT}");

    let status = match poll_status(&http, &base).await {
        Ok(s) => s,
        Err(err) => return fail(format!("{client_name}: status polling failed: {err}")),
    };

    let meta_url = format!("{base}/cl-meta.json");
    let meta: RunMeta = match http
        .get(&meta_url)
        .send()
        .await
        .and_then(|r| r.error_for_status())
    {
        Ok(r) => match r.json::<RunMeta>().await {
            Ok(m) => m,
            Err(err) => return fail(format!("{client_name}: cl-meta.json parse: {err}")),
        },
        Err(err) => return fail(format!("{client_name}: GET cl-meta.json: {err}")),
    };

    let mut overall_pass = status == "ok";
    let mut details = String::new();
    let _ = writeln!(
        details,
        "client={} source_ref={} source_sha={} client_version={} spec_tests_ref={} network={} status={}",
        meta.client,
        meta.source_ref,
        meta.source_sha,
        meta.client_version,
        meta.consensus_spec_tests_ref,
        meta.network,
        status,
    );

    if meta.suites.is_empty() {
        overall_pass = false;
        let _ = writeln!(details, "no suites declared in cl-meta.json");
    }

    for suite in &meta.suites {
        let url = format!("{base}/junit/{}", suite.junit_file);
        match http
            .get(&url)
            .send()
            .await
            .and_then(|r| r.error_for_status())
        {
            Ok(resp) => match resp.text().await {
                Ok(xml) => {
                    let summary = junit::parse(&xml);
                    let header = format_suite_header(suite);
                    let block = summary.render_details(&header, MAX_FAILED_CASES_IN_DETAILS);
                    details.push_str(&block);
                    if !summary.is_clean() {
                        overall_pass = false;
                    }
                }
                Err(err) => {
                    overall_pass = false;
                    let _ = writeln!(details, "  FAIL: {}: body read: {err}", suite.junit_file);
                }
            },
            Err(err) => {
                overall_pass = false;
                let _ = writeln!(details, "  FAIL: GET {}: {err}", suite.junit_file);
            }
        }
    }

    TestResult {
        pass: overall_pass,
        details,
    }
}

fn format_suite_header(s: &crate::meta::SuiteDescriptor) -> String {
    let mut parts = vec![s.junit_file.clone()];
    if !s.project.is_empty() {
        parts.push(format!("project={}", s.project));
    }
    if !s.preset.is_empty() {
        parts.push(format!("preset={}", s.preset));
    }
    if !s.fork.is_empty() {
        parts.push(format!("fork={}", s.fork));
    }
    if !s.category.is_empty() {
        parts.push(format!("category={}", s.category));
    }
    if let Some(sub) = &s.subcategory {
        if !sub.is_empty() {
            parts.push(format!("subcategory={sub}"));
        }
    }
    parts.join(" ")
}

async fn poll_status(http: &reqwest::Client, base: &str) -> Result<String, String> {
    let status_url = format!("{base}/status");
    let started = Instant::now();
    loop {
        if started.elapsed() > ABSOLUTE_TIMEOUT {
            return Err(format!(
                "no /status file after {} seconds",
                ABSOLUTE_TIMEOUT.as_secs()
            ));
        }
        match http.get(&status_url).send().await {
            Ok(resp) if resp.status().is_success() => match resp.text().await {
                Ok(body) => {
                    let s = body.trim().to_string();
                    if s == "ok" || s.starts_with("fail") {
                        return Ok(s);
                    }
                }
                Err(err) => {
                    tracing::debug!(error = %err, "status body read failed; retrying");
                }
            },
            Ok(_) => {
                // 404 etc. — tests still running; keep polling.
            }
            Err(err) => {
                tracing::debug!(error = %err, "status fetch failed; retrying");
            }
        }
        tokio::time::sleep(POLL_INTERVAL).await;
    }
}

fn build_env_for_client() -> HashMap<String, String> {
    let mut env = HashMap::new();
    // Tell hive's runner to wait for our results-HTTP port instead of the
    // default 8545 (EL JSON-RPC). The entrypoint only `exec`s
    // `python3 -m http.server 5151` after the native test runner finishes,
    // so this also serves as a "tests are done" signal: when start_client
    // returns, /status is already written.
    env.insert(
        "HIVE_CHECK_LIVE_PORT".to_string(),
        STATUS_PORT.to_string(),
    );
    for (sim_var, hive_var, default) in [
        ("CL_SPECS_SOURCE_REPO", "HIVE_CL_SOURCE_REPO", ""),
        ("CL_SPECS_SOURCE_REF", "HIVE_CL_SOURCE_REF", ""),
        ("CL_SPECS_TESTS_REF", "HIVE_CONSENSUS_SPEC_TESTS_REF", ""),
        // Empty default — each `<client>-specs` entrypoint applies its
        // own native default (e.g. lodestar=minimal, nimbus=minimal,
        // teku=smoke). Setting CL_SPECS_SCOPE forces all selected
        // clients to the same value (must be one each entrypoint knows).
        ("CL_SPECS_SCOPE", "HIVE_CL_SPEC_SCOPE", ""),
        ("CL_SPECS_NETWORK", "HIVE_NETWORK", "unknown"),
    ] {
        let v = env::var(sim_var).unwrap_or_else(|_| default.to_string());
        env.insert(hive_var.to_string(), v);
    }
    env
}

fn fail(details: impl Into<String>) -> TestResult {
    TestResult {
        pass: false,
        details: details.into(),
    }
}
