#!/bin/bash

set -euo pipefail

python /app/hive/prepare_leanspec_assets.py

FLAGS=(
    --genesis /tmp/lean-spec-client/genesis/config.yaml
    --listen /ip4/0.0.0.0/udp/9001/quic-v1
    --api-port 5052
    --node-id "${HIVE_NODE_ID:-lean_spec_0}"
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "${HIVE_CHECKPOINT_SYNC_URL}")
fi

if [ -n "${HIVE_LEAN_VALIDATOR_INDICES:-}" ]; then
    FLAGS+=(--validator-keys /tmp/lean-spec-client/keys)
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_BOOTNODES:-}" ]; then
    IFS=',' read -ra BOOTNODES <<< "${HIVE_BOOTNODES}"
    for bootnode in "${BOOTNODES[@]}"; do
        if [ -n "${bootnode}" ]; then
            FLAGS+=(--bootnode "${bootnode}")
        fi
    done
fi

exec uv run python -m lean_spec "${FLAGS[@]}"
