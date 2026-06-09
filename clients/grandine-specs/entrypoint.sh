#!/bin/bash
# Grandine consensus-spec-tests runner. Clones grandine (with submodules),
# fetches spec-test fixtures via grandine's own scripts/download_spec_tests.sh,
# runs cargo-nextest filtered to spec-test-bearing crates.
#
# Env:
#   HIVE_CL_SOURCE_REPO           (default: grandinetech/grandine)
#   HIVE_CL_SOURCE_REF            (default: develop)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (forwarded as SPEC_VERSION)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/grandine}"
LOG_FILE="${OUT_DIR}/grandine.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-grandinetech/grandine}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-develop}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"
log() { echo "[grandine-specs] $*" | tee -a "${LOG_FILE}"; }

log "resolved: source=${CL_SOURCE_REPO}@${CL_SOURCE_REF} scope=${SCOPE} spec-tests=${CONSENSUS_SPEC_TESTS_REF:-<pinned>} network=${NETWORK}"

run_specs() (
    set -e
    log "clone ${CL_SOURCE_REPO}@${CL_SOURCE_REF} (with submodules)"
    rm -rf "${SRC_DIR}"
    if ! git clone --depth 1 --recurse-submodules --shallow-submodules \
        --branch "${CL_SOURCE_REF}" \
        "https://github.com/${CL_SOURCE_REPO}.git" "${SRC_DIR}" 2>&1 | tee -a "${LOG_FILE}"; then
        rm -rf "${SRC_DIR}"
        git clone --recurse-submodules \
            "https://github.com/${CL_SOURCE_REPO}.git" "${SRC_DIR}" 2>&1 | tee -a "${LOG_FILE}"
        git -C "${SRC_DIR}" checkout "${CL_SOURCE_REF}" 2>&1 | tee -a "${LOG_FILE}"
        git -C "${SRC_DIR}" submodule update --init --recursive 2>&1 | tee -a "${LOG_FILE}"
    fi
    RESOLVED_SHA=$(git -C "${SRC_DIR}" rev-parse HEAD)
    log "resolved HEAD: ${RESOLVED_SHA}"

    cd "${SRC_DIR}"

    GRANDINE_VERSION=$(grep -E '^version ' Cargo.toml 2>/dev/null \
        | head -1 | sed -E 's/^version *= *"([^"]+)".*/\1/' || true)
    [[ -z "${GRANDINE_VERSION}" ]] && GRANDINE_VERSION="${CL_SOURCE_REF}"
    CLIENT_VERSION="${GRANDINE_VERSION}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

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
report-name = "grandine-spec-tests"
TOML

    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-}"
    log "download consensus-spec-tests fixtures"
    if [[ -n "${EFFECTIVE_REF}" ]]; then
        log "override SPEC_VERSION=${EFFECTIVE_REF}"
        SPEC_VERSION="${EFFECTIVE_REF}" bash scripts/download_spec_tests.sh 2>&1 | tee -a "${LOG_FILE}"
    else
        bash scripts/download_spec_tests.sh 2>&1 | tee -a "${LOG_FILE}"
    fi

    # Grandine's cargo workspace test doesn't preset-split at the build
    # layer — every spec test crate iterates both presets internally.
    # We restrict the test universe per scope by filtering test-case
    # names: the consensus-spec-tests fixtures embed the preset in their
    # path, and nextest's `-E` filter matches against the testcase name.
    NEXTEST_ARGS=(--profile cl-specs --release
                  --no-default-features
                  --features default-networks,arkworks,blst
                  --workspace
                  --exclude zkvm_host --exclude zkvm_guest_risc0
                  --exclude c_grandine --exclude csharp_grandine)
    case "${SCOPE}" in
        minimal) NEXTEST_ARGS+=(-E 'test(/minimal/)') ;;
        mainnet) NEXTEST_ARGS+=(-E 'test(/mainnet/)') ;;
        *) log "ERROR: unsupported scope '${SCOPE}'; expected 'minimal' or 'mainnet'"; exit 1 ;;
    esac

    log "cargo nextest run ${NEXTEST_ARGS[*]}"
    local rc=0
    cargo nextest run "${NEXTEST_ARGS[@]}" 2>&1 | tee -a "${LOG_FILE}" || rc=$?
    log "nextest exit=${rc}"

    HARVEST_DIR="${OUT_DIR}/junit"
    GENERATED="${SRC_DIR}/target/nextest/cl-specs/junit.xml"
    SUITES_JSON='[]'
    if [[ -f "${GENERATED}" ]]; then
        destination="${HARVEST_DIR}/grandine-spec.xml"
        cp "${GENERATED}" "${destination}"
        log "junit -> ${destination}"
        SUITES_JSON=$(jq -n --arg preset "${SCOPE}" \
            '[{junit_file:"grandine-spec.xml", project:"cargo-nextest:cl-specs",
               preset:$preset, fork:"", category:"", subcategory:null}]')
    else
        log "WARN: no JUnit produced at ${GENERATED}"
    fi

    if [[ -z "${EFFECTIVE_REF}" ]]; then
        EFFECTIVE_REF=$(cat consensus-spec-tests/.version 2>/dev/null || echo "unknown")
    fi

    jq -n \
        --arg client grandine \
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

    if [[ ${rc} -ne 0 ]]; then
        exit ${rc}
    fi
    if grep -lE '<(failure|error)\b' "${OUT_DIR}/junit/"*.xml >/dev/null 2>&1; then
        log "ERROR: at least one junit suite reports failures"
        exit 1
    fi
)
run_specs
SPECS_RC=$?

if [[ $SPECS_RC -eq 0 ]]; then
    echo "ok" > "${STATUS_FILE}"
    log "tests complete: ok"
else
    echo "failed rc=${SPECS_RC}" > "${STATUS_FILE}"
    log "tests complete: failed rc=${SPECS_RC}"
fi

exec /usr/local/bin/cl-specs-serve.sh
