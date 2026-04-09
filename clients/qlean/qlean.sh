#!/bin/bash

set -euo pipefail

QLEAN_BIN="${QLEAN_BIN:-$(command -v qlean)}"
NODE_ID="${HIVE_NODE_ID:-qlean_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
KEY_FILE=""

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

FLAGS=(
    --genesis /config/config.yaml
    --validator-registry-path /assets/lean/validators.yaml
    --validator-keys-manifest /config/hash-sig-keys/validator-keys-manifest.yaml
    --node-id "$NODE_ID"
    --data-dir /data
    --listen-addr /ip4/0.0.0.0/udp/9000/quic-v1
    --bootnodes "$BOOTNODES"
    -ldebug
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/qlean-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--prometheus-port 8080)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$QLEAN_BIN" "${FLAGS[@]}"
