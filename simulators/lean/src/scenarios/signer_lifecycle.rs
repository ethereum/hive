//! Live proposal-signer integration gaps; see docs/signer-lifecycle.md.
use std::{
    collections::HashMap,
    env, fs,
    path::PathBuf,
    process::Stdio,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

use alloy_primitives::B256;
use hivesim::{dyn_async, Client, Test};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use tokio::{io::AsyncWriteExt, process::Command};

use crate::utils::{
    signer::{SignProposalRequest, SignerOperationClient, SIGN_PROPOSAL_PATH},
    util::{
        lean_api_url, lean_clients, lean_environment, run_data_test_with_timeout,
        selected_lean_devnet, LeanDevnet, TimedDataTestSpec,
    },
};

const FIXTURES: &str = "/app/hive/signer-fixtures";
const ORACLE: &str = "/app/hive/signer_oracle.py";
const PYTHON: &str = "/app/signer-reference/.venv/bin/python";
const CLIENT_ROOT: &str = "/tmp/ream-runtime";

#[derive(Clone)]
struct Case {
    client: String,
    version: String,
    name: &'static str,
}

dyn_async! {
    pub async fn run_signer_lifecycle_suite<'a>(test: &'a mut Test, _client: Option<Client>) {
        if selected_lean_devnet() != LeanDevnet::Devnet5 {
            println!("signer-lifecycle: unsupported profile; no signer cases registered");
            return;
        }
        for client in lean_clients(test.sim.client_types().await) {
            // Only Ream's production adapter and fixture-loading contract have been audited.
            if client.name.split('_').next() != Some("ream") { continue; }
            for name in ["preparation-containment-recovery", "rejection-state-preservation"] {
                run_data_test_with_timeout(test, TimedDataTestSpec {
                    name: format!("{name} ({})", client.name),
                    description: format!("Live signer {name}: independently verified precondition and recovery; fresh disposable fixture"),
                    always_run: false, client_name: client.name.clone(),
                    timeout_duration: Duration::from_secs(480),
                    test_data: Case { client: client.name.clone(), version: client.version.clone(), name },
                }, run_case).await;
            }
        }
    }
}

fn step(id: &str, slot: u64, root: u8, expected: &str) -> Value {
    json!({"request": {"requestId": id, "validatorIndex": 0, "slot": slot,
        "blockRoot": format!("{:#x}", B256::repeat_byte(root))}, "expected": expected})
}

fn plan(name: &str, metadata: &Value) -> Vec<Value> {
    let end = metadata["prepared"][1].as_u64().expect("prepared end");
    let active_end = metadata["activation"][1].as_u64().expect("activation end");
    assert!(
        end > 2 && end < active_end,
        "fixture must straddle preparation, not activation"
    );
    match name {
        "preparation-containment-recovery" => vec![
            step("before-boundary", end - 2, 0x41, "signed"),
            // Containment contract, not an unestablished promise of automatic advancement.
            step("active-unprepared", end, 0x42, "signed_or_refused"),
            // This position is in BOTH the old window and the next window.
            step("overlap-recovery", end - 1, 0x43, "signed"),
        ],
        "rejection-state-preservation" => vec![
            step("initial-control", 3, 0x51, "signed"),
            step("inactive-request", active_end, 0x52, "refused"),
            step("after-inactive", 4, 0x53, "signed"),
            // Truncation would alias the already signed position 3.
            step("overflow-alias", (1u64 << 32) + 3, 0x54, "refused"),
            step("after-overflow", 5, 0x55, "signed"),
        ],
        _ => panic!("unknown signer scenario"),
    }
}

fn runtime_files(metadata: &Value) -> HashMap<String, Vec<u8>> {
    let mut files = HashMap::new();
    for (name, expected_hash) in metadata["fixtureHashes"]
        .as_object()
        .expect("fixture hashes")
    {
        let raw = fs::read(PathBuf::from(FIXTURES).join(name)).expect("read fixture");
        assert_eq!(
            format!("{:x}", Sha256::digest(&raw)),
            expected_hash.as_str().expect("hash"),
            "fixture changed: {name}"
        );
        files.insert(format!("{CLIENT_ROOT}/hash-sig-keys/{name}"), raw);
    }
    let genesis = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("clock")
        .as_secs()
        + 3600;
    // Future genesis prevents normal duties from consuming these test positions.
    files.insert(
        format!("{CLIENT_ROOT}/config.yaml"),
        serde_json::to_vec(&json!({
            "GENESIS_TIME": genesis, "NUM_VALIDATORS": 1, "GENESIS_VALIDATORS": [{
                "attestation_public_key": metadata["attestationPublicKey"],
                "proposal_public_key": metadata["expectedPublicKey"]
            }]
        }))
        .expect("serialize genesis"),
    );
    files.insert(format!("{CLIENT_ROOT}/validators.yaml"), serde_json::to_vec(&json!({"ream_0": [
        {"index": 0, "pubkey_hex": metadata["attestationPublicKey"], "privkey_file": "validator_0_attestation_sk.ssz"},
        {"index": 0, "pubkey_hex": metadata["expectedPublicKey"], "privkey_file": "validator_0_proposal_sk.ssz"}
    ]})).expect("serialize registry"));
    files
}

dyn_async! {
    async fn run_case<'a>(test: &'a mut Test, case: Case) {
        let mut evidence: Value = serde_json::from_slice(&fs::read(format!("{FIXTURES}/public.json")).expect("pinned signer fixture metadata")).expect("fixture JSON");
        let mut environment = lean_environment();
        environment.extend([
            ("HIVE_LEAN_TEST_DRIVER".into(), "1".into()),
            ("HIVE_BOOTNODES".into(), "none".into()),
            ("HIVE_NODE_ID".into(), "ream_0".into()),
            ("RUST_MIN_STACK".into(), "67108864".into()),
            ("TOKIO_WORKER_THREADS".into(), "2".into()),
        ]);
        let client = test.start_client_with_files(case.client.clone(), Some(environment), Some(runtime_files(&evidence))).await;
        let operation = SignerOperationClient::new(
            reqwest::Client::builder().timeout(Duration::from_secs(60)).build().expect("HTTP client"),
            lean_api_url(&client, SIGN_PROPOSAL_PATH));
        let mut steps = plan(case.name, &evidence);
        for step in &mut steps {
            let value = &step["request"];
            let request = SignProposalRequest {
                request_id: value["requestId"].as_str().expect("ID").into(), validator_index: 0,
                slot: value["slot"].as_u64().expect("slot"),
                block_root: value["blockRoot"].as_str().expect("root").parse().expect("root hex"),
            };
            match operation.sign_proposal(&request).await {
                Ok(response) => step["response"] = serde_json::to_value(response).expect("serialize response"),
                Err(error) => step["operationError"] = json!(format!("{error:#}")),
            }
            // Never stop at the negative step: recovery must actually be attempted.
        }
        evidence["steps"] = json!(steps);
        evidence["scenario"] = json!(case.name);
        evidence["clientVersion"] = json!(case.version);
        evidence["client"] = json!(case.client);
        // Preserve the public transcript even if the oracle cannot finish.
        println!("SIGNER_EVIDENCE {}", evidence);
        let mut child = Command::new(PYTHON).arg(ORACLE).stdin(Stdio::piped())
            .stdout(Stdio::piped()).stderr(Stdio::piped()).kill_on_drop(true)
            .spawn().expect("launch pinned oracle");
        child.stdin.take().expect("oracle stdin").write_all(&serde_json::to_vec(&evidence).expect("evidence JSON")).await.expect("write evidence");
        let output = tokio::time::timeout(Duration::from_secs(90), child.wait_with_output()).await
            .expect("oracle timeout").expect("oracle process");
        let report = String::from_utf8_lossy(&output.stdout);
        // Public evidence survives even if the assertion below fails.
        println!("SIGNER_REPORT {}", report);
        if let Ok(directory) = env::var("HIVE_SIGNER_EVIDENCE_DIR") {
            fs::create_dir_all(&directory).expect("evidence directory");
            fs::write(PathBuf::from(&directory).join(format!("{}.json", case.name)), serde_json::to_vec_pretty(&evidence).expect("JSON")).expect("save evidence");
        }
        assert!(output.status.success(), "signer {}: oracle exit {:?}; report {}; stderr {}", case.name, output.status.code(), report, String::from_utf8_lossy(&output.stderr));
        test.result.details = report.into_owned();
    }
}
