#!/bin/bash 
set -euo pipefail 

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}" 
ASSET_ROOT="/tmp/qlean-runtime" 
WORKSPACE="/opt/qlean" 
REAL_MODS="/opt/qlean_internal_storage" 

if [[ "$DEVNET_LABEL" == "devnet4" ]]; then 
    SOURCE_DIR="/opt/qlean-d4" 
    QLEAN_BIN_NAME="qlean-devnet4" 
    MANIFEST_NAME="validator-keys-manifest-devnet4.yaml" 
else 
    SOURCE_DIR="/opt/qlean-d3" 
    QLEAN_BIN_NAME="qlean-devnet3" 
    MANIFEST_NAME="validator-keys-manifest.yaml" 
fi 

rm -rf "$WORKSPACE" "$REAL_MODS" 
mkdir -p "$WORKSPACE/bin" "$WORKSPACE/lib" "$WORKSPACE/modules" "$REAL_MODS" 

cp "$SOURCE_DIR/bin/$QLEAN_BIN_NAME" "$WORKSPACE/bin/" 
cp "$SOURCE_DIR"/lib/*.so* "$WORKSPACE/lib/" 2>/dev/null || true 

mkdir -p "$WORKSPACE/internal_modules" 

find "$SOURCE_DIR/modules" -maxdepth 1 -type f -name "*_module.so" -exec cp {} "$REAL_MODS/" \; 

for mod in "$REAL_MODS"/*_module.so; do 
    ln -s "$mod" "$WORKSPACE/internal_modules/$(basename "$mod")" 
done 

QLEAN_BIN="$WORKSPACE/bin/$QLEAN_BIN_NAME" 

V_IDX="${HIVE_VALIDATOR_INDEX:-0}" 
RAW_ID="${HIVE_NODE_ID:-${HIVE_CLIENT_ID:-qlean_$V_IDX}}" 
CLEAN_NODE_ID=$(echo "$RAW_ID" | sed 's/^0x//') 

for var in $(env | grep '^HIVE_' | cut -d= -f1); do 
    export "$var"="${!var#0x}" 
done 

until [[ -f "$ASSET_ROOT/config.yaml" ]]; do sleep 0.5; done 

find "$ASSET_ROOT" -type f \( -name "*.yaml" -o -name "*.json" \) -exec sed -i 's/: 0x/: /g; s/\"0x/\"/g' {} + || true 

FLAGS=( 
    --modules-dir "$WORKSPACE/internal_modules" 
    --genesis "$ASSET_ROOT/config.yaml" 
    --validator-registry-path "$ASSET_ROOT/validators.yaml" 
    --validator-keys-manifest "$ASSET_ROOT/hash-sig-keys/$MANIFEST_NAME" 
    --xmss-pk "$ASSET_ROOT/hash-sig-keys/v${V_IDX}_att.pk" 
    --xmss-sk "$ASSET_ROOT/hash-sig-keys/v${V_IDX}_att.sk" 
    --data-dir "/data" 
    --node-id "$CLEAN_NODE_ID" 
    --node-key "$ASSET_ROOT/node.key" 
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
