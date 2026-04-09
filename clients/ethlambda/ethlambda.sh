#!/bin/bash

set -euo pipefail

ETHLAMBDA_BIN="${ETHLAMBDA_BIN:-$(command -v ethlambda)}"
NODE_ID="${HIVE_NODE_ID:-ethlambda_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

FLAGS=(
    --custom-network-config-dir /config
    --gossipsub-port 9000
    --node-id "$NODE_ID"
    --metrics-address 0.0.0.0
)

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/ethlambda-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics-port 8080)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$ETHLAMBDA_BIN" "${FLAGS[@]}"
