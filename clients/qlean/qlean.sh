#!/bin/bash 
set -euo pipefail 

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}" 
ASSET_ROOT="/tmp/qlean-runtime" 

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_QLEAN_BIN="/usr/local/bin/qlean-devnet3"
        ;;
    devnet4)
        DEFAULT_QLEAN_BIN="/usr/local/bin/qlean-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

QLEAN_BIN="${QLEAN_BIN:-$DEFAULT_QLEAN_BIN}"

V_IDX="${HIVE_VALIDATOR_INDEX:-0}" 
RAW_ID="${HIVE_NODE_ID:-${HIVE_CLIENT_ID:-qlean_$V_IDX}}" 
CLEAN_NODE_ID=$(echo "$RAW_ID" | sed 's/^0x//') 

for var in $(env | grep '^HIVE_' | cut -d= -f1); do 
    export "$var"="${!var#0x}" 
done 

until [[ -f "$ASSET_ROOT/config.yaml" ]]; do sleep 0.5; done 

find "$ASSET_ROOT" -type f \( -name "*.yaml" -o -name "*.json" \) -exec sed -i 's/: 0x/: /g; s/\"0x/\"/g' {} + || true 

NODE_KEY="$(cat "$ASSET_ROOT/node.key")"

FLAGS=( 
    --genesis-dir "$ASSET_ROOT"
    --data-dir "/data" 
    --node-id "$CLEAN_NODE_ID" 
    --node-key "$NODE_KEY"
    --listen-addr "/ip4/0.0.0.0/udp/9000/quic-v1" 
    --api-host "0.0.0.0" 
    --api-port 5052 
) 

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

exec "$QLEAN_BIN" "${FLAGS[@]}"
