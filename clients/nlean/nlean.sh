#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet4}"
NODE_ID="${HIVE_NODE_ID:-nlean_0}"
ASSET_ROOT="/tmp/nlean-runtime"
FORK_DIGEST="${HIVE_LEAN_FORK_DIGEST:-12345678}"
KEY_FILE=""

case "$DEVNET_LABEL" in
    devnet4)
        DEFAULT_NLEAN_BIN="/app/Lean.Client"
        ;;
    *)
        echo "Unsupported Lean devnet label for nlean: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

NLEAN_BIN="${NLEAN_BIN:-$DEFAULT_NLEAN_BIN}"

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

if [ ! -f "$ASSET_ROOT/validator-config.yaml" ]; then
    echo "Missing prepared Lean runtime assets at $ASSET_ROOT" >&2
    exit 1
fi

if [ ! -d "$ASSET_ROOT/hash-sig-keys" ]; then
    echo "Missing hash-sig-keys directory at $ASSET_ROOT/hash-sig-keys" >&2
    exit 1
fi

# nlean reads the libp2p identity via --node-key pointing at a raw hex private
# key file. Prefer HIVE_CLIENT_PRIVATE_KEY if the sim supplies one, otherwise
# fall back to the per-node key written by prepare_lean_client_assets.py.
if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/nlean-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    NODE_KEY_PATH="$KEY_FILE"
elif [ -f "$ASSET_ROOT/${NODE_ID}.key" ]; then
    NODE_KEY_PATH="$ASSET_ROOT/${NODE_ID}.key"
elif [ -f "$ASSET_ROOT/node.key" ]; then
    NODE_KEY_PATH="$ASSET_ROOT/node.key"
else
    echo "Missing libp2p node key at $ASSET_ROOT/${NODE_ID}.key or $ASSET_ROOT/node.key" >&2
    exit 1
fi

FLAGS=(
    --validator-config "$ASSET_ROOT/validator-config.yaml"
    --node "$NODE_ID"
    --data-dir /data
    --fork-digest "$FORK_DIGEST"
    --node-key "$NODE_KEY_PATH"
    --socket-port 9000
    --metrics-address 0.0.0.0
    --metrics-port 8080
    --hash-sig-key-dir "$ASSET_ROOT/hash-sig-keys"
    --api-port 5052
)

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics true)
else
    FLAGS+=(--metrics false)
fi

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_ATTESTATION_COMMITTEE_COUNT:-}" ]; then
    FLAGS+=(--attestation-committee-count "$HIVE_ATTESTATION_COMMITTEE_COUNT")
fi

if [ -n "${NLEAN_LOG_LEVEL:-}" ]; then
    FLAGS+=(--log "$NLEAN_LOG_LEVEL")
fi

exec "$NLEAN_BIN" "${FLAGS[@]}"
