#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-grandine_0}"
ASSET_ROOT="/tmp/grandine-runtime"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_GRANDINE_BIN="/usr/local/bin/grandine-devnet3"
        ;;
    devnet4)
        DEFAULT_GRANDINE_BIN="/usr/local/bin/grandine-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

GRANDINE_BIN="${GRANDINE_BIN:-$DEFAULT_GRANDINE_BIN}"

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
    --genesis "$ASSET_ROOT/config.yaml"
    --validator-registry-path "$ASSET_ROOT/validators.yaml"
    --bootnodes "$ASSET_ROOT/nodes.yaml"
    --node-id "$NODE_ID"
    --node-key "$ASSET_ROOT/node.key"
    --port 9000
    --address 0.0.0.0
    --hash-sig-key-dir "$ASSET_ROOT/hash-sig-keys"
)

export RUST_LOG="${RUST_LOG:-info}"

exec "$GRANDINE_BIN" "${FLAGS[@]}"
