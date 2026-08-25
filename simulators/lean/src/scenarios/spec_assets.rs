use std::{
    env, fs,
    path::{Path, PathBuf},
    time::Duration,
};

use crate::utils::util::{
    http_client, lean_api_url, lean_clients, lean_environment, prepare_client_runtime_files,
    run_data_test_with_timeout, selected_lean_devnet, LeanDevnet, TimedDataTestSpec,
};
use hivesim::{dyn_async, Client, Test};
use reqwest::StatusCode;
use serde::Deserialize;
use serde_json::{Map, Value};

const SPEC_TEST_ROOT_DEVNET4: &str = "/app/hive/lean-spec-tests-devnet4";
const SPEC_TEST_ROOT_DEVNET5: &str = "/app/hive/lean-spec-tests-devnet5";
const FORK_CHOICE_FIXTURE_DIRS: &[&str] = &[
    "consensus/fork_choice/lstar/fc",
    "consensus/fork_choice/lstar/fork_choice",
];
const STATE_TRANSITION_FIXTURE_DIRS: &[&str] =
    &["consensus/state_transition/lstar/state_transition"];
const VERIFY_SIGNATURES_FIXTURE_DIRS: &[&str] =
    &["consensus/verify_signatures/lstar/verify_signatures"];
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
    fn fixture_dirs(self) -> &'static [&'static str] {
        match self {
            Self::ForkChoice => FORK_CHOICE_FIXTURE_DIRS,
            Self::StateTransition => STATE_TRANSITION_FIXTURE_DIRS,
            Self::VerifySignatures => VERIFY_SIGNATURES_FIXTURE_DIRS,
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

    let spec_test_root = spec_test_root();
    let fixtures = filter_fixture_cases(discover_fixture_cases(Path::new(spec_test_root), kind));
    if fixtures.is_empty() {
        panic!(
            "No Lean {} spec-test fixtures found under {spec_test_root}",
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

fn spec_test_root() -> &'static str {
    match selected_lean_devnet() {
        LeanDevnet::Devnet4 => SPEC_TEST_ROOT_DEVNET4,
        LeanDevnet::Devnet5 => SPEC_TEST_ROOT_DEVNET5,
    }
}

fn discover_fixture_cases(root: &Path, kind: SpecFixtureKind) -> Vec<SpecFixtureCase> {
    let mut cases = Vec::new();
    for fixture_dir in kind.fixture_dirs() {
        collect_fixture_cases(&root.join(fixture_dir), kind, &mut cases);
    }
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

fn driver_step_request(step: &Value) -> Value {
    let mut request = step.clone();
    if let Some(object) = request.as_object_mut() {
        object.remove("checks");
        object.remove("storeSnapshot");
    }
    request
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
        let request = driver_step_request(step);
        let response = post_json(client, "/lean/v0/test_driver/fork_choice/step", &request).await;
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
        assert_fork_choice_checks(index, &response.snapshot, step);
    }
}

fn expects_fork_choice_init_failure(case: &Value, steps: &[Value]) -> bool {
    if !steps.is_empty() {
        return false;
    }

    expected_case_rejection(case).is_some()
        || case
            .pointer("/_info/description")
            .and_then(Value::as_str)
            .is_some_and(|description| description.contains("anchor_valid=False"))
}

fn assert_fork_choice_checks(step_index: usize, snapshot: &DriverSnapshot, step: &Value) {
    let Some(checks) = step.get("checks") else {
        return;
    };
    // `expected_store` holds fixture expectations; `snapshot` holds the client's
    // actual store state. Every assertion below is expected against actual store state.
    let expected_store = step.get("storeSnapshot");

    let assert_slot = |key: &str, actual: u64| {
        if let Some(expected) = checks.get(key).and_then(Value::as_u64) {
            assert_eq!(actual, expected, "step {step_index} {key} mismatch");
        }
    };
    assert_slot("headSlot", snapshot.head_slot);
    assert_slot("time", snapshot.time);
    assert_slot("latestJustifiedSlot", snapshot.justified_checkpoint.slot);
    assert_slot("latestFinalizedSlot", snapshot.finalized_checkpoint.slot);

    // Root checks pin a symbolic label, so the expected root comes from the
    // fixture's snapshot.
    let assert_root = |key: &str, pointer: &str, actual: &str| {
        if checks.get(key).and_then(Value::as_str).is_none() {
            return;
        }
        let Some(expected) = expected_store
            .and_then(|store| store.pointer(pointer))
            .and_then(Value::as_str)
        else {
            return;
        };
        assert_eq!(
            normalize_hex(actual),
            normalize_hex(expected),
            "step {step_index} {key} mismatch"
        );
    };
    assert_root("headRootLabel", "/headRoot", &snapshot.head_root);
    assert_root(
        "latestJustifiedRootLabel",
        "/latestJustified/root",
        &snapshot.justified_checkpoint.root,
    );
    assert_root(
        "latestFinalizedRootLabel",
        "/latestFinalized/root",
        &snapshot.finalized_checkpoint.root,
    );
    assert_root(
        "safeTargetRootLabel",
        "/safeTargetRoot",
        &snapshot.safe_target,
    );
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

    let expect_exception = expected_case_rejection(&fixture.case);
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
    let expect_exception = expected_case_rejection(&fixture.case);

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

fn expected_case_rejection(case: &Value) -> Option<&str> {
    case.get("expectException")
        .or_else(|| case.get("rejectionReason"))
        .and_then(Value::as_str)
}

#[cfg(test)]
mod tests {
    use std::{
        env, fs,
        path::{Path, PathBuf},
        time::{SystemTime, UNIX_EPOCH},
    };

    use serde_json::json;

    use crate::scenarios::spec_assets::{
        assert_fork_choice_checks, driver_step_request, DriverCheckpoint, DriverSnapshot,
    };

    use super::{
        discover_fixture_cases, expected_case_rejection, expects_fork_choice_init_failure,
        SpecFixtureKind,
    };

    fn unique_fixture_root(test_name: &str) -> PathBuf {
        let unique = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("system clock should be after the Unix epoch")
            .as_nanos();
        let root = env::temp_dir().join(format!("lean-spec-assets-{test_name}-{unique}"));
        fs::create_dir_all(&root).expect("failed to create temporary fixture root");
        root
    }

    fn write_fixture(root: &Path, relative_path: &str) {
        let path = root.join(relative_path);
        fs::create_dir_all(
            path.parent()
                .expect("fixture path should include a parent directory"),
        )
        .expect("failed to create fixture directory");
        fs::write(path, r#"{"case_a": {"steps": []}}"#).expect("failed to write fixture");
    }

    #[test]
    fn discovers_legacy_devnet4_fork_choice_fixtures() {
        let root = unique_fixture_root("legacy-fork-choice");
        write_fixture(
            &root,
            "consensus/fork_choice/lstar/fc/test_group/test_case.json",
        );

        let cases = discover_fixture_cases(&root, SpecFixtureKind::ForkChoice);

        assert_eq!(cases.len(), 1);
        assert_eq!(cases[0].test_name, "case_a");
        fs::remove_dir_all(root).expect("failed to remove temporary fixture root");
    }

    #[test]
    fn discovers_current_devnet5_fork_choice_fixtures() {
        let root = unique_fixture_root("current-fork-choice");
        write_fixture(
            &root,
            "consensus/fork_choice/lstar/fork_choice/test_group/test_case.json",
        );

        let cases = discover_fixture_cases(&root, SpecFixtureKind::ForkChoice);

        assert_eq!(cases.len(), 1);
        assert_eq!(cases[0].test_name, "case_a");
        fs::remove_dir_all(root).expect("failed to remove temporary fixture root");
    }

    #[test]
    fn discovers_current_devnet5_fixture_kinds() {
        let root = unique_fixture_root("current-kinds");
        write_fixture(
            &root,
            "consensus/fork_choice/lstar/fork_choice/test_group/test_case.json",
        );
        write_fixture(
            &root,
            "consensus/state_transition/lstar/state_transition/test_group/test_case.json",
        );
        write_fixture(
            &root,
            "consensus/verify_signatures/lstar/verify_signatures/test_group/test_case.json",
        );

        assert_eq!(
            discover_fixture_cases(&root, SpecFixtureKind::ForkChoice).len(),
            1
        );
        assert_eq!(
            discover_fixture_cases(&root, SpecFixtureKind::StateTransition).len(),
            1
        );
        assert_eq!(
            discover_fixture_cases(&root, SpecFixtureKind::VerifySignatures).len(),
            1
        );
        fs::remove_dir_all(root).expect("failed to remove temporary fixture root");
    }

    #[test]
    fn detects_legacy_and_current_expected_rejections() {
        assert_eq!(
            expected_case_rejection(&json!({ "expectException": "AssertionError" })),
            Some("AssertionError")
        );
        assert_eq!(
            expected_case_rejection(&json!({ "rejectionReason": "INVALID_BLOCK" })),
            Some("INVALID_BLOCK")
        );
        assert_eq!(expected_case_rejection(&json!({ "post": {} })), None);
    }

    #[test]
    fn detects_current_fork_choice_init_rejection() {
        assert!(expects_fork_choice_init_failure(
            &json!({ "rejectionReason": "ANCHOR_STATE_ROOT_MISMATCH" }),
            &[]
        ));
        assert!(!expects_fork_choice_init_failure(
            &json!({ "rejectionReason": "LATER_STEP_FAILURE" }),
            &[json!({ "valid": false })]
        ));
    }

    fn snapshot_fixture() -> DriverSnapshot {
        DriverSnapshot {
            head_slot: 6,
            head_root: "0xaa".to_string(),
            time: 30,
            justified_checkpoint: DriverCheckpoint {
                slot: 4,
                root: "0xbb".to_string(),
            },
            finalized_checkpoint: DriverCheckpoint {
                slot: 2,
                root: "0xcc".to_string(),
            },
            safe_target: "0xdd".to_string(),
        }
    }

    #[test]
    fn asserts_head_slot_from_fixture_key() {
        let step = json!({ "checks": {"headSlot": 6}});
        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    #[should_panic(expected = "headSlot mismatch")]
    fn reject_head_slot_divergence() {
        let step = json!({ "checks": {"headSlot": 4}});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn asserts_time_from_fixture_key() {
        let step = json!({ "checks": {"time": 30}});
        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    #[should_panic(expected = "time mismatch")]
    fn rejects_time_mismatch() {
        let step = json!({ "checks": {"time": 15}});
        
        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn asserts_latest_justified_slot_from_fixture_key() {
        let step = json!({ "checks": {"latestJustifiedSlot": 4}});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    #[should_panic(expected = "latestJustifiedSlot mismatch")]
    fn rejects_diverging_latest_justified_slot() {
        let step = json!({"checks": { "latestJustifiedSlot": 5 }});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn asserts_latest_finalized_checkpoint_from_fixture_key() {
        let step = json!({"checks": {"latestFinalizedSlot": 2}});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    #[should_panic(expected = "latestFinalizedSlot mismatch")]
    fn rejects_diverging_latest_finalized_slot() {
        let step = json!({"checks": {"latestFinalizedSlot": 3}});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn resolves_head_root_label_from_store_snapshot() {
        let step = json!({
            "checks": {"headRootLabel": "a_6"},
            "storeSnapshot": {"headRoot": "0xAA"}
        });

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    #[should_panic(expected = "headRootLabel mismatch")]
    fn rejects_diverging_head_root_label() {
        let step = json!({
            "checks": {"headRootLabel": "a_6"},
            "storeSnapshot": {"headRoot": "0xff"}
        });
        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn skips_root_labels_when_no_store_snapshot_is_present() {
        let step = json!({"checks": {"headRootLabel": "a_6"}});

        assert_fork_choice_checks(0, &snapshot_fixture(), &step);
    }

    #[test]
    fn driver_step_request_omits_fixture_expectations() {
        let step = json!({
            "stepType": "block",
            "checks": { "headSlot": 6},
            "storeSnapshot": {"headRoot": "0xaa"}
        });

        let request = driver_step_request(&step);

        assert!(request.get("stepType").is_some());
        assert!(request.get("checks").is_none());
        assert!(request.get("storeSnapshot").is_none());
    }
}
