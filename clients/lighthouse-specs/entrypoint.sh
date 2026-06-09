#!/bin/bash
# Lighthouse consensus-spec-tests runner. Clones the lighthouse source at
# HIVE_CL_SOURCE_REF, fetches consensus-spec-tests fixtures, runs
# `cargo nextest -p ef_tests` with a JUnit-emitting profile, writes results
# under /out for the cl simulator to harvest over HTTP.
#
# Env (set by simulator via hive):
#   HIVE_CL_SOURCE_REPO           (default: sigp/lighthouse)
#   HIVE_CL_SOURCE_REF            (default: stable)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (default: whatever lighthouse pins)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/lighthouse}"
LOG_FILE="${OUT_DIR}/lighthouse.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-sigp/lighthouse}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-stable}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"

log() { echo "[lighthouse-specs] $*" | tee -a "${LOG_FILE}"; }

log "resolved: source=${CL_SOURCE_REPO}@${CL_SOURCE_REF} scope=${SCOPE} spec-tests=${CONSENSUS_SPEC_TESTS_REF:-<pinned>} network=${NETWORK}"

run_specs() (
    set -e

    log "clone ${CL_SOURCE_REPO}@${CL_SOURCE_REF}"
    rm -rf "${SRC_DIR}"
    if ! git clone --depth 1 --branch "${CL_SOURCE_REF}" \
        "https://github.com/${CL_SOURCE_REPO}.git" "${SRC_DIR}" 2>&1 | tee -a "${LOG_FILE}"; then
        rm -rf "${SRC_DIR}"
        git clone "https://github.com/${CL_SOURCE_REPO}.git" "${SRC_DIR}" 2>&1 | tee -a "${LOG_FILE}"
        git -C "${SRC_DIR}" checkout "${CL_SOURCE_REF}" 2>&1 | tee -a "${LOG_FILE}"
    fi
    RESOLVED_SHA=$(git -C "${SRC_DIR}" rev-parse HEAD)
    log "resolved HEAD: ${RESOLVED_SHA}"

    cd "${SRC_DIR}"

    EF_MAKEFILE="${SRC_DIR}/testing/ef_tests/Makefile"
    [[ -f "${EF_MAKEFILE}" ]] || { log "ERROR: ${EF_MAKEFILE} missing"; exit 1; }
    PINNED_REF=$(grep -E '^CONSENSUS_SPECS_TEST_VERSION ?\??= ?' "${EF_MAKEFILE}" \
        | head -1 | sed -E 's/^[^=]*= *//')
    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-$PINNED_REF}"
    log "consensus-spec-tests pinned=${PINNED_REF} effective=${EFFECTIVE_REF}"

    CARGO_VERSION=$(grep -E '^version ?= ?"' Cargo.toml | head -1 | sed -E 's/^[^"]*"([^"]+)"$/\1/')
    [[ -z "${CARGO_VERSION}" ]] && CARGO_VERSION="${CL_SOURCE_REF}"
    CLIENT_VERSION="${CARGO_VERSION}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

    # Drop a nextest config that emits JUnit. The path 'junit.xml' is relative
    # to the nextest store root: target/nextest/cl-specs/junit.xml.
    mkdir -p "${SRC_DIR}/.config"
    cat > "${SRC_DIR}/.config/nextest.toml" <<'TOML'
[profile.cl-specs]
fail-fast = false
failure-output = "immediate"
status-level = "fail"

[profile.cl-specs.junit]
path = "junit.xml"
store-success-output = false
store-failure-output = true
report-name = "lighthouse-ef-tests"
TOML

    log "download consensus-spec-tests fixtures (${EFFECTIVE_REF})"
    CONSENSUS_SPECS_TEST_VERSION="${EFFECTIVE_REF}" \
        make -C "${SRC_DIR}/testing/ef_tests" 2>&1 | tee -a "${LOG_FILE}"

    SUITES_JSON='[]'

    run_nextest() {
        local label="$1" features="$2" filter_expr="${3:-}" suite_basename="$4"
        local preset="$5" fork="$6" category="$7" subcategory="$8"
        log "nextest [${label}] (features=${features})"
        local rc=0
        ( cd "${SRC_DIR}" \
            && cargo nextest run \
                --profile cl-specs \
                --release \
                -p ef_tests \
                --features "${features}" \
                ${filter_expr:+-E "${filter_expr}"} \
        ) 2>&1 | tee -a "${LOG_FILE}" || rc=$?
        local generated="${SRC_DIR}/target/nextest/cl-specs/junit.xml"
        local destination="${OUT_DIR}/junit/${suite_basename}"
        if [[ -f "${generated}" ]]; then
            cp "${generated}" "${destination}"
            log "nextest [${label}] exit=${rc} junit -> ${destination}"
            SUITES_JSON=$(jq --arg jf "${suite_basename}" \
                --arg project "ef_tests:${features}" \
                --arg preset "${preset}" \
                --arg fork "${fork}" \
                --arg category "${category}" \
                --arg subcategory "${subcategory}" \
                '. + [{junit_file:$jf, project:$project, preset:$preset, fork:$fork, category:$category,
                       subcategory:(if $subcategory == "" then null else $subcategory end)}]' \
                <<<"${SUITES_JSON}")
        else
            log "WARN: nextest [${label}] produced no JUnit at ${generated}"
        fi
    }

    # Lighthouse's ef_tests crate runs every preset that the spec-test
    # fixtures provide; preset is selected per-fixture rather than
    # globally. We map the minimal/mainnet scope onto crypto-backend
    # feature toggles, matching `make run-ef-tests`: minimal runs the
    # fast `fake_crypto` pass; mainnet runs the real-BLS pass.
    case "${SCOPE}" in
        minimal)
            run_nextest "fake_crypto" \
                "ef_tests,fake_crypto" \
                "" \
                "lighthouse-fake_crypto.xml" "minimal" "" "all" ""
            ;;
        mainnet)
            run_nextest "real_crypto" \
                "ef_tests" \
                "" \
                "lighthouse-real_crypto.xml" "mainnet" "" "all" ""
            ;;
        *)
            log "ERROR: unsupported scope '${SCOPE}'; expected 'minimal' or 'mainnet'"; exit 1 ;;
    esac

    jq -n \
        --arg client lighthouse \
        --arg source_repo "${CL_SOURCE_REPO}" \
        --arg source_ref "${CL_SOURCE_REF}" \
        --arg source_sha "${RESOLVED_SHA}" \
        --arg client_version "${CLIENT_VERSION}" \
        --arg consensus_spec_tests_ref "${EFFECTIVE_REF}" \
        --arg network "${NETWORK}" \
        --argjson suites "${SUITES_JSON}" \
        '{client:$client, source_repo:$source_repo, source_ref:$source_ref, source_sha:$source_sha,
          client_version:$client_version, consensus_spec_tests_ref:$consensus_spec_tests_ref,
          network:$network, suites:$suites}' > "${META_FILE}"
    cat "${META_FILE}"

    # Fail the run if any of the JUnit XMLs report failures.
    if grep -lE '<(failure|error)\b' "${OUT_DIR}/junit/"*.xml >/dev/null 2>&1; then
        log "ERROR: at least one junit suite reports failures"
        exit 1
    fi
)
SPECS_RC=$?

if [[ $SPECS_RC -eq 0 ]]; then
    echo "ok" > "${STATUS_FILE}"
    log "tests complete: ok"
else
    echo "failed rc=${SPECS_RC}" > "${STATUS_FILE}"
    log "tests complete: failed rc=${SPECS_RC}"
fi

exec /usr/local/bin/cl-specs-serve.sh
