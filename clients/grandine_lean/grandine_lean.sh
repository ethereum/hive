#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-grandine_lean_0}"
ASSET_ROOT="/tmp/grandine_lean-runtime"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-$ASSET_ROOT/config.yaml}"
LEAN_SPEC_SOURCE_PEER_ID="16Uiu2HAmHzBkRq62mG95vsjKMuYQBezZCtjPXYWUoyVxMxi71aB3"

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

if [ "$DEVNET_LABEL" = "devnet4" ]; then
    export HIVE_LEAN_GRANDINE_SKIP_XMSS_VERIFY="${HIVE_LEAN_GRANDINE_SKIP_XMSS_VERIFY:-1}"
fi

if [ "$DEVNET_LABEL" = "devnet3" ]; then
    export HIVE_LEAN_GRANDINE_SKIP_XMSS_VERIFY="${HIVE_LEAN_GRANDINE_SKIP_XMSS_VERIFY:-1}"
    export HIVE_LEAN_GRANDINE_REQUIRE_KEYS="${HIVE_LEAN_GRANDINE_REQUIRE_KEYS:-1}"
fi

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

checkpoint_bootnode_multiaddr() {
    local checkpoint_url host_port host

    checkpoint_url="${HIVE_CHECKPOINT_SYNC_URL:-}"
    if [ -z "$checkpoint_url" ]; then
        return 1
    fi

    host_port="${checkpoint_url#http://}"
    host_port="${host_port#https://}"
    host_port="${host_port%%/*}"
    host="${host_port%%:*}"
    if [ -z "$host" ]; then
        return 1
    fi

    printf "/ip4/%s/udp/9001/quic-v1/p2p/%s" "$host" "$LEAN_SPEC_SOURCE_PEER_ID"
}

checkpoint_source_host_port() {
    local checkpoint_url host_port host port

    checkpoint_url="${HIVE_CHECKPOINT_SYNC_URL:-}"
    if [ -z "$checkpoint_url" ]; then
        return 1
    fi

    host_port="${checkpoint_url#http://}"
    host_port="${host_port#https://}"
    host_port="${host_port%%/*}"

    if [ -z "$host_port" ]; then
        return 1
    fi

    host="${host_port%%:*}"
    if [ "$host" = "$host_port" ]; then
        case "$checkpoint_url" in
            https://*)
                port="443"
                ;;
            *)
                port="80"
                ;;
        esac
    else
        port="${host_port##*:}"
    fi

    if [ -z "$host" ] || [ -z "$port" ]; then
        return 1
    fi

    printf "%s %s\n" "$host" "$port"
}

normalize_bootnode() {
    local bootnode="$1"

    if [ "$DEVNET_LABEL" = "devnet3" ] && [[ "$bootnode" == enr:* ]]; then
        checkpoint_bootnode_multiaddr && return 0
    fi

    printf "%s" "$bootnode"
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
    --node-id "$NODE_ID"
    --address 0.0.0.0
    --port 9000
    --discovery-port 9001
    --http-address 0.0.0.0
    --http-port 5052
)

FLAGS+=(--validator-registry-path "$VALIDATOR_REGISTRY_PATH")

if [ -d "$ASSET_ROOT/hash-sig-keys" ]; then
    FLAGS+=(--hash-sig-key-dir "$ASSET_ROOT/hash-sig-keys")
fi

if [ -n "${HIVE_BOOTNODES:-}" ] && [ "${HIVE_BOOTNODES}" != "none" ]; then
    IFS=',' read -r -a bootnode_entries <<< "${HIVE_BOOTNODES}"
    for bootnode in "${bootnode_entries[@]}"; do
        FLAGS+=(--bootnodes "$(normalize_bootnode "$bootnode")")
    done
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

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    if read -r checkpoint_host checkpoint_port < <(checkpoint_source_host_port); then
        checkpoint_wait_secs="${HIVE_LEAN_GRANDINE_CHECKPOINT_WAIT_SECS:-30}"
        checkpoint_deadline=$((SECONDS + checkpoint_wait_secs))
        while ! (exec 3<>"/dev/tcp/${checkpoint_host}/${checkpoint_port}") >/dev/null 2>&1; do
            if [ "$SECONDS" -ge "$checkpoint_deadline" ]; then
                echo "Checkpoint source ${checkpoint_host}:${checkpoint_port} did not become reachable within ${checkpoint_wait_secs}s" >&2
                exit 1
            fi
            sleep 1
        done
    fi
fi

exec "$LEAN_BIN" "${FLAGS[@]}"
