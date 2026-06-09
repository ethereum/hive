#!/bin/bash
# Prysm consensus-spec-tests runner. Clones prysm source, runs `bazel test`
# against //testing/spectest, harvests per-target test.xml from
# bazel-testlogs.
#
# Note: prysm's Bazel WORKSPACE SHA-pins consensus-spec-tests; we cannot
# override the ref without recomputing the SHA. The cl-meta.json records
# the requested ref as `consensus_spec_tests_ref` when set, with a clear
# label so reviewers can see when the actual ref baked into the WORKSPACE
# diverges.
#
# Env:
#   HIVE_CL_SOURCE_REPO           (default: OffchainLabs/prysm)
#   HIVE_CL_SOURCE_REF            (default: develop)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (informational; not honoured)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/prysm}"
LOG_FILE="${OUT_DIR}/prysm.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-OffchainLabs/prysm}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-develop}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"
log() { echo "[prysm-specs] $*" | tee -a "${LOG_FILE}"; }

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

    PRYSM_VERSION=$(grep -E '^\s*version\s*=' runtime/version/version.go 2>/dev/null \
        | head -1 | sed -E 's/.*"([^"]+)".*/\1/' || true)
    [[ -z "${PRYSM_VERSION}" ]] && PRYSM_VERSION="${CL_SOURCE_REF}"
    CLIENT_VERSION="${PRYSM_VERSION}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

    bazel --version 2>&1 | tee -a "${LOG_FILE}" || true

    # Prysm's spectest tree splits at the package level by preset.
    case "${SCOPE}" in
        minimal) BAZEL_TARGETS="//testing/spectest/minimal/..." ;;
        mainnet) BAZEL_TARGETS="//testing/spectest/mainnet/..." ;;
        *) log "ERROR: unsupported scope '${SCOPE}'; expected 'minimal' or 'mainnet'"; exit 1 ;;
    esac

    log "bazel test ${BAZEL_TARGETS}"
    local rc=0
    bazel test \
        --test_output=errors \
        --keep_going \
        ${BAZEL_TARGETS} 2>&1 | tee -a "${LOG_FILE}" || rc=$?
    log "bazel exit=${rc}"

    HARVEST_DIR="${OUT_DIR}/junit"
    TESTLOGS_DIR="${SRC_DIR}/bazel-testlogs"
    SUITES_JSON='[]'
    local count=0
    if [[ -e "${TESTLOGS_DIR}" ]]; then
        real_root=$(readlink -f "${TESTLOGS_DIR}" 2>/dev/null || echo "${TESTLOGS_DIR}")
        while IFS= read -r xml; do
            rel="${xml#${real_root}/}"
            rel="${rel#${TESTLOGS_DIR}/}"
            flat=$(echo "${rel}" | tr '/' '_')
            cp "${xml}" "${HARVEST_DIR}/${flat}"
            count=$((count+1))
        done < <(find -L "${TESTLOGS_DIR}" -name "test.xml" -type f 2>/dev/null)
    fi
    log "harvested ${count} JUnit file(s)"

    shopt -s nullglob
    for f in "${HARVEST_DIR}"/*.xml; do
        base=$(basename "${f}")
        case "${base}" in
            *general_bls*)         cat="bls";         preset="" ;;
            *general_ssz_generic*) cat="ssz_generic"; preset="" ;;
            *general_kzg*)         cat="kzg";         preset="" ;;
            *general*)             cat="";            preset="" ;;
            *minimal*)             cat="";            preset="minimal" ;;
            *mainnet*)             cat="";            preset="mainnet" ;;
            *)                     cat="";            preset="" ;;
        esac
        SUITES_JSON=$(jq --arg jf "${base}" \
            --arg category "${cat}" \
            --arg preset "${preset}" \
            '. + [{junit_file:$jf, project:"prysm:bazel-spectest", preset:$preset, fork:"", category:$category, subcategory:null}]' \
            <<<"${SUITES_JSON}")
    done
    shopt -u nullglob

    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-unknown}"

    jq -n \
        --arg client prysm \
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
