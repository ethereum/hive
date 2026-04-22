#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-ethlambda_0}"
ASSET_ROOT="/tmp/ethlambda-runtime"
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

materialize_runtime_local_ip

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
