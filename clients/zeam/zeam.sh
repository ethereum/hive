#!/bin/bash

set -euo pipefail

ZEAM_BIN="${ZEAM_BIN:-/app/zig-out/bin/zeam}"
NODE_ID="${HIVE_NODE_ID:-zeam_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

FLAGS=(
    node
    --custom_genesis /assets/lean
    --validator_config /assets/lean/validators.yaml
    --data-dir /data
    --node-id "$NODE_ID"
    --api-port 5052
)

if [ "$BOOTNODES" != "none" ]; then
    FLAGS+=(--bootnodes "$BOOTNODES")
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/zeam-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics_enable)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$ZEAM_BIN" "${FLAGS[@]}"
