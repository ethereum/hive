#!/bin/bash
# Teku consensus-spec-tests runner. Clones teku source, runs gradle's
# `:eth-reference-tests:referenceTest` task with the wrapper gradlew,
# harvests per-test-class JUnit XMLs from build/test-results.
#
# Env:
#   HIVE_CL_SOURCE_REPO           (default: Consensys/teku)
#   HIVE_CL_SOURCE_REF            (default: master)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (optional override; patches build.gradle in place)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/teku}"
LOG_FILE="${OUT_DIR}/teku.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-Consensys/teku}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-master}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"
log() { echo "[teku-specs] $*" | tee -a "${LOG_FILE}"; }

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

    TEKU_VERSION=$(grep -E '^version ' build.gradle 2>/dev/null \
        | head -1 | sed -E "s/^version *= *'(.+)'.*/\1/" || true)
    [[ -z "${TEKU_VERSION}" ]] && TEKU_VERSION="${CL_SOURCE_REF}"
    CLIENT_VERSION="${TEKU_VERSION}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

    PINNED_REF=$(grep -E '^def refTestVersion ' build.gradle 2>/dev/null \
        | sed -E 's/.*"([^"]+)".*/\1/' | tail -1)
    log "pinned refTestVersion=${PINNED_REF}"
    if [[ -n "${CONSENSUS_SPEC_TESTS_REF}" && "${CONSENSUS_SPEC_TESTS_REF}" != "${PINNED_REF}" ]]; then
        log "overriding refTestVersion: ${PINNED_REF} -> ${CONSENSUS_SPEC_TESTS_REF}"
        sed -i.bak -E \
            "s|(def refTestVersion = nightly \\? \"nightly\" : \")[^\"]+(\")|\\1${CONSENSUS_SPEC_TESTS_REF}\\2|" \
            build.gradle
    fi
    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-$PINNED_REF}"

    # Teku's gradle reference-tests task is not preset-split — it builds
    # the full reference-tests source tree, which contains both minimal
    # and mainnet test classes mixed together. We filter test classes by
    # the `_minimal_` / `_mainnet_` package convention used in the
    # generated reference-test source; this is a best-effort match.
    GRADLE_TASK=":eth-reference-tests:referenceTest"
    GRADLE_FILTER=()
    case "${SCOPE}" in
        minimal) GRADLE_FILTER+=(--tests "*minimal*") ;;
        mainnet) GRADLE_FILTER+=(--tests "*mainnet*") ;;
        *) log "ERROR: unsupported scope '${SCOPE}'; expected 'minimal' or 'mainnet'"; exit 1 ;;
    esac

    # Run expandRefTests then referenceTest separately to keep gradle 9's
    # cross-task input validator happy.
    local rc=0
    log "gradle expandRefTests"
    ./gradlew --no-daemon --console=plain expandRefTests 2>&1 | tee -a "${LOG_FILE}" || rc=$?
    if [[ ${rc} -eq 0 ]]; then
        log "gradle ${GRADLE_TASK} ${GRADLE_FILTER[*]-}"
        ./gradlew --no-daemon --console=plain "${GRADLE_TASK}" "${GRADLE_FILTER[@]}" 2>&1 | tee -a "${LOG_FILE}" || rc=$?
    fi
    log "gradle exit=${rc}"

    HARVEST_DIR="${OUT_DIR}/junit"
    SUITES_JSON='[]'
    shopt -s globstar nullglob
    local count=0
    for xml in "${SRC_DIR}"/**/build/test-results/referenceTest*/TEST-*.xml; do
        rel="${xml#${SRC_DIR}/}"
        flat=$(echo "${rel}" | tr '/' '_')
        cp "${xml}" "${HARVEST_DIR}/${flat}"
        count=$((count+1))
    done
    log "harvested ${count} JUnit file(s) -> ${HARVEST_DIR}"

    SUITE_PRESET="${SCOPE}"
    SUITE_CATEGORY=""
    for f in "${HARVEST_DIR}"/*.xml; do
        base=$(basename "${f}")
        SUITES_JSON=$(jq --arg jf "${base}" \
            --arg category "${SUITE_CATEGORY}" \
            --arg preset "${SUITE_PRESET}" \
            '. + [{junit_file:$jf, project:"eth-reference-tests:referenceTest",
                   preset:$preset, fork:"", category:$category, subcategory:null}]' \
            <<<"${SUITES_JSON}")
    done
    shopt -u globstar nullglob

    jq -n \
        --arg client teku \
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
(
    run_specs
    SPECS_RC=$?
    
    if [[ $SPECS_RC -eq 0 ]]; then
        echo "ok" > "${STATUS_FILE}"
        log "tests complete: ok"
    else
        echo "failed rc=${SPECS_RC}" > "${STATUS_FILE}"
        log "tests complete: failed rc=${SPECS_RC}"
    fi
    
) &

exec /usr/local/bin/cl-specs-serve.sh
