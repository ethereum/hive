#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-lantern_0}"
ASSET_ROOT="/tmp/lantern-runtime"
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
        DEFAULT_LANTERN_BIN="/opt/lantern-devnet3/bin/lantern"
        export LD_LIBRARY_PATH="/opt/lantern-devnet3/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
        DEFAULT_GOSSIP_TOPIC="devnet0"
        ;;
    devnet4)
        DEFAULT_LANTERN_BIN="/opt/lantern/bin/lantern"
        DEFAULT_GOSSIP_TOPIC="12345678"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

LANTERN_BIN="${LANTERN_BIN:-$DEFAULT_LANTERN_BIN}"
GOSSIP_TOPIC="${HIVE_LEAN_FORK_DIGEST:-$DEFAULT_GOSSIP_TOPIC}"

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
    --data-dir /data
    --genesis-config "$ASSET_ROOT/config.yaml"
    --validator-registry-path "$ASSET_ROOT/validators.yaml"
    --nodes-path "$ASSET_ROOT/nodes.yaml"
    --validator-config "$ASSET_ROOT/validator-config.yaml"
    --node-id "$NODE_ID"
    --node-key-path "$ASSET_ROOT/node.key"
    --listen-address "/ip4/0.0.0.0/udp/9000/quic-v1"
    --http-port 5052
    --metrics-port 8080
    --hash-sig-key-dir "$ASSET_ROOT/hash-sig-keys"
    --log-level debug
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

FLAGS+=(--devnet "$GOSSIP_TOPIC")

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$LANTERN_BIN" "${FLAGS[@]}"
