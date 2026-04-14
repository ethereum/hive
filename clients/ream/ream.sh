#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-ream_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
ASSET_ROOT="/tmp/ream-runtime"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-$ASSET_ROOT/config.yaml}"
VALIDATOR_REGISTRY_PATH="${HIVE_LEAN_VALIDATOR_REGISTRY_PATH:-$ASSET_ROOT/validators.yaml}"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_REAM_BIN="/usr/local/bin/ream-devnet3"
        ;;
    devnet4)
        DEFAULT_REAM_BIN="/usr/local/bin/ream-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

REAM_BIN="${REAM_BIN:-$DEFAULT_REAM_BIN}"

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

if [ ! -f "$NETWORK_CONFIG" ]; then
    echo "Missing prepared Lean network config at $NETWORK_CONFIG" >&2
    exit 1
fi

if [ ! -f "$VALIDATOR_REGISTRY_PATH" ]; then
    echo "Missing prepared Lean validator registry at $VALIDATOR_REGISTRY_PATH" >&2
    exit 1
fi

FLAGS=(
    --network "$NETWORK_CONFIG"
    --validator-registry-path "$VALIDATOR_REGISTRY_PATH"
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
elif [ -f "$ASSET_ROOT/node.key" ]; then
    KEY_FILE="$(mktemp /tmp/ream-node-key.XXXXXX)"
    cat "$ASSET_ROOT/node.key" > "$KEY_FILE"
    FLAGS+=(--private-key-path "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics --metrics-address 0.0.0.0)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$REAM_BIN" --ephemeral lean_node "${FLAGS[@]}"
