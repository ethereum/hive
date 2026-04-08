#!/bin/bash

set -euo pipefail

REAM_BIN="${REAM_BIN:-$(command -v ream)}"
NODE_ID="${HIVE_NODE_ID:-ream_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

FLAGS=(
    --network ephemery
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
