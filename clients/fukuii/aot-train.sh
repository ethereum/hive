#!/bin/bash
# Image-build-time AOT training run (JDK 25, JEP 483/514). Invoked once by the
# Dockerfile; never at hive run time.
#
# Boots fukuii through the real hive entry point (/fukuii.sh, same JVM flags, same
# -jar path) with a post-merge Prague genesis like the consume-* simulators hand it,
# waits for :8545 as hive does, drives one Engine API block-building round
# trip so the request path is recorded too, then stops the node with SIGTERM. On exit
# the JVM writes /app/fukuii/fukuii.aot (-XX:AOTCacheOutput, set by fukuii.sh when
# FUKUII_AOT_TRAIN is non-empty).
#
# aot-train-genesis.json is the pre-state of the ethereum-tests mergeExample_Prague
# fixture (deposit, EIP-4788/2935/7002/7251 system contracts) plus one funded account,
# so the Prague block build/import below runs the system calls a real fixture does.
#
# Everything the run writes (datadir, node key, logs, genesis, JWT, temp files) is
# removed afterwards, so the image's runtime state is identical to an untrained one:
# the only artefact is the cache file.
#
# Failing to boot or failing to produce the cache fails the image build — a silent
# fall-back to "no cache" would quietly give the startup time back.
set -euo pipefail

AOT_CACHE=/app/fukuii/fukuii.aot
LOG=/tmp/aot-train.log
TIMEOUT=180

# hive-like environment: post-merge, every fork through Prague active at genesis.
export HIVE_CHAIN_ID=1 HIVE_NETWORK_ID=1
for f in HOMESTEAD TANGERINE SPURIOUS BYZANTIUM CONSTANTINOPLE PETERSBURG ISTANBUL \
         MUIR_GLACIER BERLIN LONDON ARROW_GLACIER GRAY_GLACIER; do
    export "HIVE_FORK_$f=0"
done
export HIVE_MERGE_BLOCK_ID=0 HIVE_TERMINAL_TOTAL_DIFFICULTY=0
export HIVE_SHANGHAI_TIMESTAMP=0 HIVE_CANCUN_TIMESTAMP=0 HIVE_PRAGUE_TIMESTAMP=0
# The consume-*/rpc-compat readiness port (libhive default). fukuii.sh turns it into
# fukuii.network.readiness-port, so StdNode binds 8551 then 8545 — the same order those
# simulators get.
export HIVE_CHECK_LIVE_PORT=8545

cp /aot-train-genesis.json /genesis.json
rm -f "$AOT_CACHE"
TMP_BEFORE=$(ls -A /tmp)   # the run extracts native libs etc. into /tmp; see step 4

FUKUII_AOT_TRAIN=1 /fukuii.sh >"$LOG" 2>&1 &
PID=$!   # fukuii.sh execs java, so this is the JVM

fail() {
    echo "aot-train: $*" >&2
    kill -9 "$PID" 2>/dev/null || true
    tail -n 80 "$LOG" >&2 || true
    exit 1
}

rpc() {  # rpc <url> <method> <params-json> [auth-header]
    curl -sS --max-time 30 -X POST -H 'Content-Type: application/json' ${4:+-H "$4"} \
        -d "{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"$2\",\"params\":$3}" "$1"
}

b64url() { base64 -w0 | tr '+/' '-_' | tr -d '='; }

jwt() {  # HS256 over the fixed hive secret fukuii.sh writes to /jwtsecret
    local key header payload sig
    key=$(sed 's/^0x//' /jwtsecret)
    header=$(printf '{"alg":"HS256","typ":"JWT"}' | b64url)
    payload=$(printf '{"iat":%d}' "$(date +%s)" | b64url)
    sig=$(printf '%s.%s' "$header" "$payload" \
        | openssl dgst -sha256 -mac HMAC -macopt "hexkey:$key" -binary | b64url)
    printf '%s.%s.%s' "$header" "$payload" "$sig"
}

# 1. Startup path: wait for :8545 like hive's HIVE_CHECK_LIVE_PORT probe.
for _ in $(seq 1 $((TIMEOUT * 10))); do
    kill -0 "$PID" 2>/dev/null || fail "node exited before :8545 bound"
    if (exec 3<>/dev/tcp/127.0.0.1/8545) 2>/dev/null; then break; fi
    sleep 0.1
done
(exec 3<>/dev/tcp/127.0.0.1/8545) 2>/dev/null || fail ":8545 did not bind within ${TIMEOUT}s"

rpc http://127.0.0.1:8545 eth_blockNumber '[]' | jq -e '.result == "0x0"' >/dev/null \
    || fail "eth_blockNumber did not return 0x0"
/hive-bin/enode.sh >/dev/null || fail "enode.sh failed"

# 2. First-request path: build and import one block over the Engine API, the way the
#    consume-engine simulator drives a client. Best effort — a failure here only means
#    fewer classes are cached, so it is reported but does not fail the build.
engine_round_trip() {
    local e=http://127.0.0.1:8551 genesis ts fcu pid payload
    genesis=$(rpc http://127.0.0.1:8545 eth_getBlockByNumber '["0x0",false]' | jq -er .result.hash)
    rpc http://127.0.0.1:8545 eth_chainId '[]' >/dev/null
    rpc http://127.0.0.1:8545 eth_getBalance '["0x7e5f4552091a69125d5dfcb7b8c2659029395bdf","latest"]' >/dev/null
    rpc "$e" engine_exchangeCapabilities '[["engine_newPayloadV4"]]' "Authorization: Bearer $(jwt)" \
        | jq -e .result >/dev/null
    ts=$(printf '0x%x' 12)
    fcu="[{\"headBlockHash\":\"$genesis\",\"safeBlockHash\":\"$genesis\",\"finalizedBlockHash\":\"$genesis\"},
          {\"timestamp\":\"$ts\",\"prevRandao\":\"0x$(printf '%064x' 0)\",
           \"suggestedFeeRecipient\":\"0x$(printf '%040x' 0)\",\"withdrawals\":[],
           \"parentBeaconBlockRoot\":\"0x$(printf '%064x' 0)\"}]"
    pid=$(rpc "$e" engine_forkchoiceUpdatedV3 "$fcu" "Authorization: Bearer $(jwt)" | jq -er .result.payloadId)
    payload=$(rpc "$e" engine_getPayloadV4 "[\"$pid\"]" "Authorization: Bearer $(jwt)" | jq -ec .result)
    rpc "$e" engine_newPayloadV4 \
        "[$(jq -c .executionPayload <<<"$payload"),[],\"0x$(printf '%064x' 0)\",$(jq -c .executionRequests <<<"$payload")]" \
        "Authorization: Bearer $(jwt)" | jq -e '.result.status == "VALID"' >/dev/null
}
if engine_round_trip; then
    echo "aot-train: engine round trip OK"
else
    echo "aot-train: WARNING engine round trip failed; cache covers startup only" >&2
fi

# 3. Clean stop. SIGTERM runs the JVM's normal exit path, which is where the AOT
#    cache is assembled; SIGKILL would lose it.
kill -TERM "$PID"
wait "$PID" || true   # SIGTERM exit status (143) is expected

[ -s "$AOT_CACHE" ] || fail "JVM exited without writing $AOT_CACHE"
grep -E '\[aot' "$LOG" | tail -n 5 || true
echo "aot-train: wrote $AOT_CACHE ($(du -h "$AOT_CACHE" | cut -f1))"

# 4. Leave no training state behind. fukuii.sh recreates all of these at run time.
rm -rf /app/data/* /app/hive-conf/* /genesis.json /jwtsecret /aot-train-genesis.json "$LOG"
for f in $(ls -A /tmp); do
    grep -qxF "$f" <<<"$TMP_BEFORE" || rm -rf "/tmp/$f"
done
