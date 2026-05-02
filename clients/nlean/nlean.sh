#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet4}"
NODE_ID="${HIVE_NODE_ID:-nlean_0}"
ASSET_ROOT="/tmp/nlean-runtime"
FORK_DIGEST="${HIVE_LEAN_FORK_DIGEST:-12345678}"
KEY_FILE=""
LOCAL_IP_PLACEHOLDER="__HIVE_LOCAL_IP__"

detect_local_ip() {
    hostname -i 2>/dev/null | tr ' ' '\n' | grep -E '^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$' | grep -v '^127\.' | head -n 1
}

materialize_runtime_local_ip() {
    local runtime_ip

    if [ ! -f "$ASSET_ROOT/validator-config.yaml" ]; then
        return
    fi

    if ! grep -q "$LOCAL_IP_PLACEHOLDER" "$ASSET_ROOT/validator-config.yaml"; then
        return
    fi

    runtime_ip="$(detect_local_ip || true)"
    if [ -z "$runtime_ip" ]; then
        echo "Unable to resolve local container IP for $NODE_ID" >&2
        exit 1
    fi

    sed -i "s/${LOCAL_IP_PLACEHOLDER}/${runtime_ip}/g" "$ASSET_ROOT/validator-config.yaml"
}

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

if [ ! -f "$ASSET_ROOT/annotated_validators.yaml" ]; then
    echo "Missing prepared Lean runtime assets at $ASSET_ROOT" >&2
    exit 1
fi

if [ ! -f "$ASSET_ROOT/validator-config.yaml" ]; then
    echo "Missing prepared Lean validator config at $ASSET_ROOT/validator-config.yaml" >&2
    exit 1
fi

if [ ! -f "$ASSET_ROOT/config.yaml" ]; then
    echo "Missing prepared Lean network config at $ASSET_ROOT/config.yaml" >&2
    exit 1
fi

if [ ! -f "$ASSET_ROOT/nodes.yaml" ]; then
    echo "Missing prepared Lean bootnodes config at $ASSET_ROOT/nodes.yaml" >&2
    exit 1
fi

if [ ! -d "$ASSET_ROOT/hash-sig-keys" ]; then
    echo "Missing hash-sig-keys directory at $ASSET_ROOT/hash-sig-keys" >&2
    exit 1
fi

materialize_runtime_local_ip

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
    --custom-network-config-dir "$ASSET_ROOT"
    --node "$NODE_ID"
    --data-dir /data
    --network "$FORK_DIGEST"
    --node-key "$NODE_KEY_PATH"
    --socket-port 9000
    --metrics-address 0.0.0.0
    --metrics-port 8080
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
