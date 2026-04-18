#!/bin/bash
# TODO: verify against gean CLI — flags were translated from ream.sh against
# gean's cmd/gean/main.go (github.com/geanlabs/gean). gean consumes a single
# --custom-network-config-dir directory expected to contain config.yaml,
# nodes.yaml (bootnodes), annotated_validators.yaml, and hash-sig-keys/; it
# has no --bootnodes flag and always starts its metrics server. Re-verify
# filenames and HIVE_BOOTNODES / HIVE_METRICS_ENABLED wiring once the
# hive lean simulator provisions the matching asset layout.

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-gean_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
ASSET_ROOT="/tmp/gean-runtime"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-$ASSET_ROOT/config.yaml}"
VALIDATOR_REGISTRY_PATH="${HIVE_LEAN_VALIDATOR_REGISTRY_PATH:-$ASSET_ROOT/validators.yaml}"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_GEAN_BIN="/usr/local/bin/gean-devnet3"
        ;;
    devnet4)
        DEFAULT_GEAN_BIN="/usr/local/bin/gean-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

GEAN_BIN="${GEAN_BIN:-$DEFAULT_GEAN_BIN}"

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

# gean reads its inputs from a directory, not individual file paths.
CONFIG_DIR="$(dirname "$NETWORK_CONFIG")"

# HIVE_BOOTNODES is informational here — gean loads bootnodes from
# "$CONFIG_DIR/nodes.yaml", which the lean simulator is expected to write.
echo "gean: node=$NODE_ID devnet=$DEVNET_LABEL bootnodes=$BOOTNODES config_dir=$CONFIG_DIR" >&2

FLAGS=(
    --custom-network-config-dir "$CONFIG_DIR"
    --node-id "$NODE_ID"
    --gossipsub-port 9000
    --http-address 0.0.0.0
    --api-port 5052
    --metrics-port 8080
    --data-dir /data
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/gean-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
elif [ -f "$ASSET_ROOT/node.key" ]; then
    KEY_FILE="$(mktemp /tmp/gean-node-key.XXXXXX)"
    cat "$ASSET_ROOT/node.key" > "$KEY_FILE"
    FLAGS+=(--node-key "$KEY_FILE")
fi

# gean starts the metrics server unconditionally; HIVE_METRICS_ENABLED=0
# would require upstream gating. For now we always bind to 0.0.0.0:8080.
if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    :
fi

export GOLOG_LOG_LEVEL="${GOLOG_LOG_LEVEL:-info}"

exec "$GEAN_BIN" "${FLAGS[@]}"
