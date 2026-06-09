#!/bin/bash
# Nimbus consensus-spec-tests runner. Bootstraps nimbus-eth2's vendored nim
# compiler via `make update`, builds the per-preset
# `consensus_spec_tests_<preset>` binary, runs it with --xml output, writes
# results under /out for the cl simulator to harvest over HTTP.
#
# Env:
#   HIVE_CL_SOURCE_REPO           (default: status-im/nimbus-eth2)
#   HIVE_CL_SOURCE_REF            (default: unstable)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (forwarded as CONSENSUS_TEST_VECTOR_VERSIONS)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/nimbus-eth2}"
LOG_FILE="${OUT_DIR}/nimbus.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-status-im/nimbus-eth2}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-unstable}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"
log() { echo "[nimbus-specs] $*" | tee -a "${LOG_FILE}"; }

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

    NIMBUS_VERSION_RAW=$(grep -E 'versionMajor|versionMinor|versionBuild' beacon_chain/version.nim 2>/dev/null \
        | sed -E 's/.*= *([0-9]+).*/\1/' | tr '\n' '.' || true)
    [[ -z "${NIMBUS_VERSION_RAW}" ]] && NIMBUS_VERSION_RAW="${CL_SOURCE_REF}"
    CLIENT_VERSION="${NIMBUS_VERSION_RAW%.}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

    if [[ -n "${CONSENSUS_SPEC_TESTS_REF}" ]]; then
        export CONSENSUS_TEST_VECTOR_VERSIONS="${CONSENSUS_SPEC_TESTS_REF}"
        log "forwarding fixtures override: CONSENSUS_TEST_VECTOR_VERSIONS=${CONSENSUS_TEST_VECTOR_VERSIONS}"
    fi

    log "nimbus toolchain bootstrap (make update — slow)"
    make -j"$(nproc)" update 2>&1 | tee -a "${LOG_FILE}"

    log "download consensus-spec-tests fixtures"
    scripts/setup_scenarios.sh fixturesCache 2>&1 | tee -a "${LOG_FILE}"

    SUITES_JSON='[]'

    run_preset() {
        local preset="$1" suite_basename="$2"
        local binary="consensus_spec_tests_${preset}"
        log "build ${binary}"
        make -j"$(nproc)" \
            DISABLE_TEST_FIXTURES_SCRIPT=1 \
            "${binary}" 2>&1 | tee -a "${LOG_FILE}"

        local rc=0
        log "run ${binary}"
        ./build/"${binary}" --xml:"./build/${binary}.xml" --console 2>&1 | tee -a "${LOG_FILE}" || rc=$?
        log "${binary} exit=${rc}"

        local generated="${SRC_DIR}/build/${binary}.xml"
        local destination="${OUT_DIR}/junit/${suite_basename}"
        if [[ -f "${generated}" ]]; then
            cp "${generated}" "${destination}"
            log "junit -> ${destination}"
            SUITES_JSON=$(jq --arg jf "${suite_basename}" \
                --arg project "${binary}" \
                --arg preset "${preset}" \
                '. + [{junit_file:$jf, project:$project, preset:$preset, fork:"", category:"", subcategory:null}]' \
                <<<"${SUITES_JSON}")
        else
            log "WARN: no JUnit produced at ${generated}"
        fi
    }

    case "${SCOPE}" in
        minimal) run_preset minimal "nimbus-presets-minimal.xml" ;;
        mainnet) run_preset mainnet "nimbus-presets-mainnet.xml" ;;
        full)
            run_preset minimal "nimbus-presets-minimal.xml"
            run_preset mainnet "nimbus-presets-mainnet.xml"
            ;;
        *)
            log "ERROR: unknown HIVE_CL_SPEC_SCOPE: ${SCOPE}"; exit 1 ;;
    esac

    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-}"
    if [[ -z "${EFFECTIVE_REF}" ]]; then
        DV_SCRIPT="${SRC_DIR}/vendor/nim-eth2-scenarios/download_test_vectors.sh"
        if [[ -f "${DV_SCRIPT}" ]]; then
            EFFECTIVE_REF=$(grep -E 'VERSIONS=\(' "${DV_SCRIPT}" | head -1 \
                | sed -E 's/.*VERSIONS=\("([^"]+)".*/\1/' || true)
        fi
        [[ -z "${EFFECTIVE_REF}" ]] && EFFECTIVE_REF="unknown"
    fi
    log "effective consensus-spec-tests ref=${EFFECTIVE_REF}"

    jq -n \
        --arg client nimbus \
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
