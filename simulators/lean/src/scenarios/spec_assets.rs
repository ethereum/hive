use std::{
    env, fs,
    path::{Path, PathBuf},
    time::Duration,
};

use crate::utils::util::{
    http_client, lean_api_url, lean_clients, lean_environment, prepare_client_runtime_files,
    run_data_test_with_timeout, TimedDataTestSpec,
};
use hivesim::{dyn_async, Client, Test};
use reqwest::StatusCode;
use serde::Deserialize;
use serde_json::{Map, Value};

const SPEC_TEST_ROOT: &str = "/app/hive/lean-spec-tests";
const FORK_CHOICE_FIXTURE_DIR: &str = "consensus/fork_choice/lstar/fc";
const STATE_TRANSITION_FIXTURE_DIR: &str = "consensus/state_transition/lstar/state_transition";
const VERIFY_SIGNATURES_FIXTURE_DIR: &str = "consensus/verify_signatures/lstar/verify_signatures";
const SPEC_ASSET_TEST_TIMEOUT: Duration = Duration::from_secs(120);
const SPEC_ASSET_FILTER_ENV: &str = "HIVE_LEAN_SPEC_ASSET_FILTER";
const SPEC_ASSET_LIMIT_ENV: &str = "HIVE_LEAN_SPEC_ASSET_LIMIT";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum SpecFixtureKind {
    ForkChoice,
    StateTransition,
    VerifySignatures,
}

impl SpecFixtureKind {
    fn fixture_dir(self) -> &'static str {
        match self {
            Self::ForkChoice => FORK_CHOICE_FIXTURE_DIR,
            Self::StateTransition => STATE_TRANSITION_FIXTURE_DIR,
            Self::VerifySignatures => VERIFY_SIGNATURES_FIXTURE_DIR,
        }
    }

    fn family(self) -> &'static str {
        match self {
            Self::ForkChoice => "fork_choice",
            Self::StateTransition => "state_transition",
            Self::VerifySignatures => "verify_signatures",
        }
    }
}

#[derive(Clone)]
struct SpecFixtureCase {
    client_name: String,
    kind: SpecFixtureKind,
    path: PathBuf,
    test_name: String,
    case: Value,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DriverStepResponse {
    accepted: bool,
    error: Option<String>,
    snapshot: DriverSnapshot,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DriverSnapshot {
    head_slot: u64,
    head_root: String,
    time: u64,
    justified_checkpoint: DriverCheckpoint,
    finalized_checkpoint: DriverCheckpoint,
    safe_target: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct DriverCheckpoint {
    slot: u64,
    root: String,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StateTransitionResponse {
    succeeded: bool,
    error: Option<String>,
    post: Option<StateTransitionPost>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct VerifySignaturesResponse {
    succeeded: bool,
    error: Option<String>,
}

#[derive(Debug, Deserialize)]
#[serde(rename_all = "camelCase")]
struct StateTransitionPost {
    slot: u64,
    latest_block_header_slot: u64,
    latest_block_header_state_root: String,
    historical_block_hashes_count: usize,
}

dyn_async! {
    pub async fn run_spec_assets_fork_choice_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        run_spec_assets_lean_test_suite_for_kind(test, SpecFixtureKind::ForkChoice).await;
    }
}

dyn_async! {
    pub async fn run_spec_assets_state_transition_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        run_spec_assets_lean_test_suite_for_kind(test, SpecFixtureKind::StateTransition).await;
    }
}

dyn_async! {
    pub async fn run_spec_assets_verify_signatures_lean_test_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        run_spec_assets_lean_test_suite_for_kind(test, SpecFixtureKind::VerifySignatures).await;
    }
}

async fn run_spec_assets_lean_test_suite_for_kind(test: &mut Test, kind: SpecFixtureKind) {
    let clients = lean_clients(test.sim.client_types().await);
    if clients.is_empty() {
        panic!("No lean clients were selected for this run");
    }

    let fixtures = filter_fixture_cases(discover_fixture_cases(Path::new(SPEC_TEST_ROOT), kind));
    if fixtures.is_empty() {
        panic!(
            "No Lean {} spec-test fixtures found under {SPEC_TEST_ROOT}",
            kind.family()
        );
    }

    for client in &clients {
        for fixture in &fixtures {
            let mut fixture = fixture.clone();
            fixture.client_name = client.name.clone();
            let name = hive_test_name(&fixture);
            let description = format!("Lean spec-test fixture: {}", fixture.path.display());
            run_data_test_with_timeout(
                test,
                TimedDataTestSpec {
                    name,
                    description,
                    always_run: false,
                    client_name: client.name.clone(),
                    timeout_duration: SPEC_ASSET_TEST_TIMEOUT,
                    test_data: fixture,
                },
                run_spec_fixture_case,
            )
            .await;
        }
    }
}

dyn_async! {
    async fn run_spec_fixture_case<'a>(test: &'a mut Test, fixture: SpecFixtureCase) {
        let mut environment = lean_environment();
        environment.insert("HIVE_LEAN_TEST_DRIVER".to_string(), "1".to_string());
        environment.insert("HIVE_BOOTNODES".to_string(), "none".to_string());
        let files = prepare_client_runtime_files(&fixture.client_name, &environment)
            .unwrap_or_else(|err| panic!("failed to prepare client files for {}: {err}", fixture.client_name));
        let client = test
            .start_client_with_files(fixture.client_name.clone(), Some(environment), Some(files))
            .await;

        match fixture.kind {
            SpecFixtureKind::ForkChoice => run_fork_choice_fixture(&client, &fixture).await,
            SpecFixtureKind::StateTransition => run_state_transition_fixture(&client, &fixture).await,
            SpecFixtureKind::VerifySignatures => run_verify_signatures_fixture(&client, &fixture).await,
        }
    }
}

fn discover_fixture_cases(root: &Path, kind: SpecFixtureKind) -> Vec<SpecFixtureCase> {
    let mut cases = Vec::new();
    collect_fixture_cases(&root.join(kind.fixture_dir()), kind, &mut cases);
    cases.sort_by(|a, b| a.path.cmp(&b.path).then(a.test_name.cmp(&b.test_name)));
    cases
}

fn filter_fixture_cases(mut cases: Vec<SpecFixtureCase>) -> Vec<SpecFixtureCase> {
    if let Ok(filter) = env::var(SPEC_ASSET_FILTER_ENV) {
        let filter = filter.trim().to_ascii_lowercase();
        if !filter.is_empty() {
            cases.retain(|case| {
                case.path
                    .display()
                    .to_string()
                    .to_ascii_lowercase()
                    .contains(&filter)
                    || case.test_name.to_ascii_lowercase().contains(&filter)
            });
        }
    }

    if let Ok(limit) = env::var(SPEC_ASSET_LIMIT_ENV) {
        let limit = limit
            .trim()
            .parse::<usize>()
            .unwrap_or_else(|err| panic!("Invalid {SPEC_ASSET_LIMIT_ENV} value {limit:?}: {err}"));
        cases.truncate(limit);
    }

    cases
}

fn collect_fixture_cases(dir: &Path, kind: SpecFixtureKind, cases: &mut Vec<SpecFixtureCase>) {
    let Ok(entries) = fs::read_dir(dir) else {
        return;
    };

    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_dir() {
            collect_fixture_cases(&path, kind, cases);
            continue;
        }
        if path.extension().and_then(|extension| extension.to_str()) != Some("json") {
            continue;
        }

        let content = fs::read_to_string(&path)
            .unwrap_or_else(|err| panic!("failed to read fixture {}: {err}", path.display()));
        let fixture: Map<String, Value> = serde_json::from_str(&content)
            .unwrap_or_else(|err| panic!("failed to parse fixture {}: {err}", path.display()));
        for (test_name, case) in fixture {
            cases.push(SpecFixtureCase {
                client_name: String::new(),
                kind,
                path: path.clone(),
                test_name,
                case,
            });
        }
    }
}

fn hive_test_name(fixture: &SpecFixtureCase) -> String {
    let family = fixture.kind.family();
    let stem = fixture
        .path
        .file_stem()
        .and_then(|stem| stem.to_str())
        .unwrap_or("unknown_fixture");
    let parent = fixture
        .path
        .parent()
        .and_then(|parent| parent.file_name())
        .and_then(|parent| parent.to_str())
        .unwrap_or("unknown_group");
    format!("spec-assets/{family}/{parent}/{stem}")
}

async fn post_json(client: &Client, path: &str, payload: &Value) -> reqwest::Response {
    let response = post_json_raw(client, path, payload).await;
    assert!(
        response.status().is_success(),
        "POST {} returned HTTP {}",
        lean_api_url(client, path),
        response.status()
    );
    response
}

async fn post_json_raw(client: &Client, path: &str, payload: &Value) -> reqwest::Response {
    let http = http_client();
    let url = lean_api_url(client, path);
    http.post(url.clone())
        .json(payload)
        .send()
        .await
        .unwrap_or_else(|err| panic!("failed to POST {url}: {err}"))
}

async fn run_fork_choice_fixture(client: &Client, fixture: &SpecFixtureCase) {
    let steps = fixture
        .case
        .get("steps")
        .and_then(Value::as_array)
        .expect("fork-choice fixture missing steps array");

    let init = serde_json::json!({
        "anchorState": fixture.case.get("anchorState").expect("fork-choice fixture missing anchorState"),
        "anchorBlock": fixture.case.get("anchorBlock").expect("fork-choice fixture missing anchorBlock"),
        "genesisTime": fixture.case.pointer("/anchorState/config/genesisTime").and_then(Value::as_u64),
    });
    let response = post_json_raw(client, "/lean/v0/test_driver/fork_choice/init", &init).await;
    let expects_init_failure = expects_fork_choice_init_failure(&fixture.case, steps);
    if !response.status().is_success() {
        assert!(
            expects_init_failure,
            "POST {} returned HTTP {}",
            lean_api_url(client, "/lean/v0/test_driver/fork_choice/init"),
            response.status()
        );
        return;
    }
    assert_eq!(response.status(), StatusCode::NO_CONTENT);
    assert!(
        !expects_init_failure,
        "fork-choice init unexpectedly accepted invalid anchor fixture"
    );

    for (index, step) in steps.iter().enumerate() {
        let response = post_json(client, "/lean/v0/test_driver/fork_choice/step", step).await;
        let response: DriverStepResponse = response.json().await.unwrap_or_else(|err| {
            panic!("failed to decode fork-choice step response at step {index}: {err}")
        });
        if let Some(expected_valid) = step.get("valid").and_then(Value::as_bool) {
            assert_eq!(
                response.accepted, expected_valid,
                "step {index} acceptance mismatch; driver error: {:?}",
                response.error
            );
        }
        if let Some(checks) = step.get("checks") {
            assert_fork_choice_checks(index, &response.snapshot, checks);
        }
    }
}

fn expects_fork_choice_init_failure(case: &Value, steps: &[Value]) -> bool {
    steps.is_empty()
        && case
            .pointer("/_info/description")
            .and_then(Value::as_str)
            .is_some_and(|description| description.contains("anchor_valid=False"))
}

fn assert_fork_choice_checks(step_index: usize, snapshot: &DriverSnapshot, checks: &Value) {
    if let Some(expected) = checks.get("headSlot").and_then(Value::as_u64) {
        assert_eq!(
            snapshot.head_slot, expected,
            "step {step_index} headSlot mismatch"
        );
    }
    if let Some(expected) = checks.get("headRoot").and_then(Value::as_str) {
        assert_eq!(
            normalize_hex(&snapshot.head_root),
            normalize_hex(expected),
            "step {step_index} headRoot mismatch"
        );
    }
    if let Some(expected) = checks.get("time").and_then(Value::as_u64) {
        assert_eq!(snapshot.time, expected, "step {step_index} time mismatch");
    }
    if let Some(expected) = checks
        .pointer("/justifiedCheckpoint/slot")
        .and_then(Value::as_u64)
    {
        assert_eq!(
            snapshot.justified_checkpoint.slot, expected,
            "step {step_index} justified slot mismatch"
        );
    }
    if let Some(expected) = checks
        .pointer("/justifiedCheckpoint/root")
        .and_then(Value::as_str)
    {
        assert_eq!(
            normalize_hex(&snapshot.justified_checkpoint.root),
            normalize_hex(expected),
            "step {step_index} justified root mismatch"
        );
    }
    if let Some(expected) = checks
        .pointer("/finalizedCheckpoint/slot")
        .and_then(Value::as_u64)
    {
        assert_eq!(
            snapshot.finalized_checkpoint.slot, expected,
            "step {step_index} finalized slot mismatch"
        );
    }
    if let Some(expected) = checks
        .pointer("/finalizedCheckpoint/root")
        .and_then(Value::as_str)
    {
        assert_eq!(
            normalize_hex(&snapshot.finalized_checkpoint.root),
            normalize_hex(expected),
            "step {step_index} finalized root mismatch"
        );
    }
    if let Some(expected) = checks.get("safeTarget").and_then(Value::as_str) {
        assert_eq!(
            normalize_hex(&snapshot.safe_target),
            normalize_hex(expected),
            "step {step_index} safeTarget mismatch"
        );
    }
}

async fn run_state_transition_fixture(client: &Client, fixture: &SpecFixtureCase) {
    let response = post_json(
        client,
        "/lean/v0/test_driver/state_transition/run",
        &fixture.case,
    )
    .await;
    let response: StateTransitionResponse = response
        .json()
        .await
        .unwrap_or_else(|err| panic!("failed to decode state-transition response: {err}"));

    let expect_exception = fixture.case.get("expectException").and_then(Value::as_str);
    assert_eq!(
        response.succeeded,
        expect_exception.is_none(),
        "state-transition success mismatch; expected exception: {:?}; driver error: {:?}",
        expect_exception,
        response.error
    );

    if let Some(expected_post) = fixture.case.get("post") {
        let post = response
            .post
            .as_ref()
            .expect("successful transition should return post summary");
        if let Some(expected) = expected_post.get("slot").and_then(Value::as_u64) {
            assert_eq!(post.slot, expected, "post.slot mismatch");
        }
        if let Some(expected) = expected_post
            .get("latestBlockHeaderSlot")
            .and_then(Value::as_u64)
        {
            assert_eq!(
                post.latest_block_header_slot, expected,
                "post.latestBlockHeaderSlot mismatch"
            );
        }
        if let Some(expected) = expected_post
            .get("latestBlockHeaderStateRoot")
            .and_then(Value::as_str)
        {
            assert_eq!(
                normalize_hex(&post.latest_block_header_state_root),
                normalize_hex(expected),
                "post.latestBlockHeaderStateRoot mismatch"
            );
        }
        if let Some(expected) = expected_post
            .get("historicalBlockHashesCount")
            .and_then(Value::as_u64)
        {
            assert_eq!(
                post.historical_block_hashes_count as u64, expected,
                "post.historicalBlockHashesCount mismatch"
            );
        }
    }
}

async fn run_verify_signatures_fixture(client: &Client, fixture: &SpecFixtureCase) {
    let response = post_json(
        client,
        "/lean/v0/test_driver/verify_signatures/run",
        &fixture.case,
    )
    .await;
    let response: VerifySignaturesResponse = response
        .json()
        .await
        .unwrap_or_else(|err| panic!("failed to decode verify-signatures response: {err}"));
    let expect_exception = fixture.case.get("expectException").and_then(Value::as_str);

    assert_eq!(
        response.succeeded,
        expect_exception.is_none(),
        "verify-signatures success mismatch; expected exception: {:?}; driver error: {:?}",
        expect_exception,
        response.error
    );
}

fn normalize_hex(value: &str) -> String {
    value.trim_start_matches("0x").to_ascii_lowercase()
}
