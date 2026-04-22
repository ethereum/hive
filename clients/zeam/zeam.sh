#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet4}"
NODE_ID="${HIVE_NODE_ID:-zeam_0}"
ASSET_ROOT="/tmp/zeam-runtime"
LOCAL_IP_PLACEHOLDER="__HIVE_LOCAL_IP__"

detect_local_ip() {
    hostname -i 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | grep -v '^127\.' | head -n 1
}

materialize_runtime_local_ip() {
    local runtime_ip

    if [ ! -f "$ASSET_ROOT/validator-config.yaml" ]; then
        return
    fi

    runtime_ip="$(detect_local_ip)"
    if [ -z "$runtime_ip" ]; then
        echo "Unable to resolve local container IP for $NODE_ID" >&2
        exit 1
    fi

    sed -i "s/${LOCAL_IP_PLACEHOLDER}/${runtime_ip}/g" "$ASSET_ROOT/validator-config.yaml"
}

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

materialize_runtime_local_ip

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
    CHECKPOINT_SYNC_URL="$HIVE_CHECKPOINT_SYNC_URL"
    CHECKPOINT_SYNC_SUFFIX="/lean/v0/states/finalized"

    if [[ "$CHECKPOINT_SYNC_URL" == *"$CHECKPOINT_SYNC_SUFFIX" ]]; then
        CHECKPOINT_SYNC_URL="${CHECKPOINT_SYNC_URL%"$CHECKPOINT_SYNC_SUFFIX"}"
    fi

    FLAGS+=(--checkpoint-sync-url "$CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$ZEAM_BIN" "${FLAGS[@]}"
