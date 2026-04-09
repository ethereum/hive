#!/bin/bash

set -euo pipefail

LANTERN_BIN="${LANTERN_BIN:-$(command -v lantern || command -v lantern_cli)}"
NODE_ID="${HIVE_NODE_ID:-lantern_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

FLAGS=(
    --data-dir /data
    --validator-registry-path /assets/lean/validators.yaml
    --node-id "$NODE_ID"
    --listen-address "/ip4/0.0.0.0/udp/9000/quic-v1"
    --http-port 5052
    --log-level debug
)

if [ -n "${HIVE_NETWORK:-}" ]; then
    FLAGS+=(--devnet "$HIVE_NETWORK")
else
    FLAGS+=(--devnet ephemery) 
fi

if [ "$BOOTNODES" != "none" ]; then
    FLAGS+=(--bootnodes "$BOOTNODES")
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/lantern-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key-path "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics-port 8080)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$LANTERN_BIN" "${FLAGS[@]}"