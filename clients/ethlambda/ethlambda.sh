#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-ethlambda_0}"
ASSET_ROOT="/tmp/ethlambda-runtime"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_ETHLAMBDA_BIN="/usr/local/bin/ethlambda-devnet3"
        ;;
    devnet4)
        DEFAULT_ETHLAMBDA_BIN="/usr/local/bin/ethlambda-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

ETHLAMBDA_BIN="${ETHLAMBDA_BIN:-$DEFAULT_ETHLAMBDA_BIN}"

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
    --custom-network-config-dir "$ASSET_ROOT"
    --gossipsub-port 9000
    --http-address 0.0.0.0
    --api-port 5052
    --metrics-port 8080
    --node-key "$ASSET_ROOT/node.key"
    --node-id "$NODE_ID"
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$ETHLAMBDA_BIN" "${FLAGS[@]}"
