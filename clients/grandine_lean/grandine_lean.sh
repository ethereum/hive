#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-grandine_lean_0}"
ASSET_ROOT="/tmp/grandine_lean-runtime"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-$ASSET_ROOT/config.yaml}"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_LEAN_BIN="/usr/local/bin/lean_client-devnet3"
        VALIDATOR_REGISTRY_PATH="${HIVE_LEAN_VALIDATOR_REGISTRY_PATH:-$ASSET_ROOT/validators.yaml}"
        ;;
    devnet4)
        DEFAULT_LEAN_BIN="/usr/local/bin/lean_client-devnet4"
        VALIDATOR_REGISTRY_PATH="${HIVE_LEAN_VALIDATOR_REGISTRY_PATH:-$ASSET_ROOT/annotated_validators.yaml}"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

LEAN_BIN="${LEAN_BIN:-$DEFAULT_LEAN_BIN}"

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
    --genesis "$NETWORK_CONFIG"
    --validator-registry-path "$VALIDATOR_REGISTRY_PATH"
    --node-id "$NODE_ID"
    --address 0.0.0.0
    --port 9000
    --discovery-port 9001
    --http-address 0.0.0.0
    --http-port 5052
)

if [ "$DEVNET_LABEL" = "devnet4" ]; then
    FLAGS+=(--hash-sig-key-dir "$ASSET_ROOT/hash-sig-keys")
fi

if [ -n "${HIVE_BOOTNODES:-}" ] && [ "${HIVE_BOOTNODES}" != "none" ]; then
    FLAGS+=(--bootnodes "$HIVE_BOOTNODES")
fi

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/grandine_lean-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics --metrics-address 0.0.0.0 --metrics-port 8080)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$LEAN_BIN" "${FLAGS[@]}"
