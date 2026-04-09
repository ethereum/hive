#!/bin/bash

set -euo pipefail

REAM_BIN="${REAM_BIN:-$(command -v ream)}"
NODE_ID="${HIVE_NODE_ID:-ream_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-ephemery}"

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

if [ -n "${HIVE_LEAN_GENESIS_TIME:-}" ]; then
    mkdir -p /tmp/ream-network
    cat > /tmp/ream-network/config.yaml <<EOF
GENESIS_TIME: ${HIVE_LEAN_GENESIS_TIME}
NUM_VALIDATORS: 3
GENESIS_VALIDATORS:
  - "0x4ebeee6aa607247dadcba631f8ffb473e170c9000181261de029a149a86d825c60c3f626315c9b549d425f2f6d51d94bcbe14a02"
  - "0x7017345bbe0feb6b7a926e1f0b988158b7552e81fbbaebd43a87a0cb679f247987f275ba0ca85df1131669f210b5e777a3797618"
  - "0x1859df2e6b17775d054a696990aa680b5dcfdfcd5c063b9c6b7f90ed4da48d2a8a503f73574ddde2b0409e148dc42129980ba1af"
EOF
    NETWORK_CONFIG="/tmp/ream-network/config.yaml"
fi

FLAGS=(
    --network "$NETWORK_CONFIG"
    --validator-registry-path /assets/lean/validators.yaml
    --node-id "$NODE_ID"
    --socket-address 0.0.0.0
    --socket-port 9000
    --http-address 0.0.0.0
    --http-port 5052
    --bootnodes "$BOOTNODES"
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/ream-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--private-key-path "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics --metrics-address 0.0.0.0)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$REAM_BIN" --ephemeral lean_node "${FLAGS[@]}"
