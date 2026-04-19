#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet4}"
NODE_ID="${HIVE_NODE_ID:-zeam_0}"
ASSET_ROOT="/tmp/zeam-runtime"

case "$DEVNET_LABEL" in
    devnet4)
        DEFAULT_ZEAM_BIN="/usr/local/bin/zeam"
        ;;
    *)
        echo "Unsupported Lean devnet label for zeam: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

ZEAM_BIN="${ZEAM_BIN:-$DEFAULT_ZEAM_BIN}"

cleanup() {
    if [ -d "$ASSET_ROOT" ]; then
        rm -rf "$ASSET_ROOT"
    fi
}

trap cleanup EXIT

if [ ! -f "$ASSET_ROOT/config.yaml" ]; then
    echo "Missing prepared Lean runtime assets at $ASSET_ROOT" >&2
    exit 1
fi

FLAGS=(
    node
    --custom_genesis "$ASSET_ROOT"
    --validator_config "$ASSET_ROOT"
    --data-dir /data
    --node-id "$NODE_ID"
    --api-port 5052
    --metrics_enable
    --metrics-port 8080
    --node-key "$ASSET_ROOT/node.key"
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$ZEAM_BIN" "${FLAGS[@]}"
