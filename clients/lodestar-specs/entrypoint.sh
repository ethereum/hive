#!/bin/bash
# Lodestar consensus-spec-tests runner. Clones the lodestar source at
# HIVE_CL_SOURCE_REF, builds with pnpm, runs vitest spec projects with the
# JUnit reporter, writes results under /out for the cl simulator to harvest
# over HTTP.
#
# Env:
#   HIVE_CL_SOURCE_REPO           (default: ChainSafe/lodestar)
#   HIVE_CL_SOURCE_REF            (default: unstable)
#   HIVE_CONSENSUS_SPEC_TESTS_REF (default: whatever lodestar pins)
#   HIVE_CL_SPEC_SCOPE            minimal|mainnet (default: minimal)
#   HIVE_NETWORK                  devnet label, recorded in cl-meta.json

set -uo pipefail

OUT_DIR="${OUT_DIR:-/out}"
SRC_DIR="${SRC_DIR:-/work/lodestar}"
LOG_FILE="${OUT_DIR}/lodestar.log"
META_FILE="${OUT_DIR}/cl-meta.json"
STATUS_FILE="${OUT_DIR}/status"

CL_SOURCE_REPO="${HIVE_CL_SOURCE_REPO:-${HIVE_CL_SOURCE_REPO_DEFAULT:-ChainSafe/lodestar}}"
CL_SOURCE_REF="${HIVE_CL_SOURCE_REF:-${HIVE_CL_SOURCE_REF_DEFAULT:-unstable}}"
CONSENSUS_SPEC_TESTS_REF="${HIVE_CONSENSUS_SPEC_TESTS_REF:-}"
NETWORK="${HIVE_NETWORK:-unknown}"
SCOPE="${HIVE_CL_SPEC_SCOPE:-minimal}"

mkdir -p "${OUT_DIR}/junit"
touch "${LOG_FILE}"
log() { echo "[lodestar-specs] $*" | tee -a "${LOG_FILE}"; }

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

    PIN_FILE="${SRC_DIR}/spec-tests-version.json"
    [[ -f "${PIN_FILE}" ]] || { log "ERROR: ${PIN_FILE} missing"; exit 1; }
    PINNED_REF=$(jq -r '.ethereumConsensusSpecsTests.specVersion' "${PIN_FILE}")
    EFFECTIVE_REF="${CONSENSUS_SPEC_TESTS_REF:-$PINNED_REF}"
    log "consensus-spec-tests pinned=${PINNED_REF} effective=${EFFECTIVE_REF}"
    if [[ -n "${CONSENSUS_SPEC_TESTS_REF}" && "${CONSENSUS_SPEC_TESTS_REF}" != "${PINNED_REF}" ]]; then
        log "overriding lodestar's pinned spec-tests version"
        jq --arg v "${CONSENSUS_SPEC_TESTS_REF}" \
            '.ethereumConsensusSpecsTests.specVersion = $v' \
            "${PIN_FILE}" > "${PIN_FILE}.new"
        mv "${PIN_FILE}.new" "${PIN_FILE}"
    fi

    CLIENT_VERSION=$(jq -r .version package.json 2>/dev/null || echo "")
    [[ -z "${CLIENT_VERSION}" || "${CLIENT_VERSION}" == "null" ]] && CLIENT_VERSION="${CL_SOURCE_REF}"
    CLIENT_VERSION="${CLIENT_VERSION}-${RESOLVED_SHA:0:8}"
    log "client_version=${CLIENT_VERSION}"

    PACKAGE_MANAGER=$(jq -r '.packageManager // empty' package.json)
    [[ -n "${PACKAGE_MANAGER}" ]] \
        && corepack prepare "${PACKAGE_MANAGER}" --activate 2>&1 | tee -a "${LOG_FILE}" \
        || corepack prepare pnpm@latest --activate 2>&1 | tee -a "${LOG_FILE}"
    pnpm --version | tee -a "${LOG_FILE}"

    log "pnpm install"
    pnpm install --frozen-lockfile 2>&1 | tee -a "${LOG_FILE}"
    log "pnpm build"
    pnpm build 2>&1 | tee -a "${LOG_FILE}"
    log "pnpm download-spec-tests"
    pnpm download-spec-tests 2>&1 | tee -a "${LOG_FILE}"

    BEACON_DIR="${SRC_DIR}/packages/beacon-node"
    SUITES_JSON='[]'

    vitest_run() {
        local label="$1" junit_basename="$2" project="$3" preset="$4"
        local fork="$5" category="$6" path="$7" node_opts="${8:-}"
        local junit_path="${OUT_DIR}/junit/${junit_basename}"
        log "vitest [${label}]"
        local rc=0
        ( cd "${BEACON_DIR}" \
            && NODE_OPTIONS="${node_opts}" \
               pnpm exec vitest run --project "${project}" "${path}" \
                --reporter=junit \
                --outputFile="${junit_path}" \
        ) 2>&1 | tee -a "${LOG_FILE}" || rc=$?
        log "vitest [${label}] exit=${rc} junit=${junit_path}"
        if [[ -f "${junit_path}" ]]; then
            SUITES_JSON=$(jq --arg jf "${junit_basename}" \
                --arg project "${project}" \
                --arg preset "${preset}" \
                --arg fork "${fork}" \
                --arg category "${category}" \
                --arg source_subdir "packages/beacon-node" \
                '. + [{junit_file:$jf, project:$project, preset:$preset, fork:$fork, category:$category,
                       subcategory:null, source_subdir:$source_subdir}]' \
                <<<"${SUITES_JSON}")
        else
            log "WARN: no JUnit file produced for ${label}"
        fi
    }

    case "${SCOPE}" in
        minimal)
            vitest_run "presets/minimal" "lodestar-presets-minimal.xml" spec-minimal minimal "" sanity test/spec/presets/
            ;;
        mainnet)
            vitest_run "presets/mainnet" "lodestar-presets-mainnet.xml" spec-mainnet mainnet "" sanity test/spec/presets/ "--max-old-space-size=4096"
            ;;
        *)
            log "ERROR: unsupported scope '${SCOPE}'; expected 'minimal' or 'mainnet'"; exit 1 ;;
    esac

    jq -n \
        --arg client lodestar \
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
