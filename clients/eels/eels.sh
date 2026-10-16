#!/bin/bash

# Startup script to initialize and boot the EELS Engine API server.
#
# This script assumes the following files:
#  - `/execution-specs` contains the EELS checkout with uv environment
#  - `/genesis.json` file is located in the filesystem root (mandatory)
#
# This script assumes the following environment variables:
#
#  - HIVE_CHAIN_ID                network ID number to use for the eth protocol
#  - HIVE_<FORK>_TIMESTAMP        fork schedule, read directly by the server
#
# EELS executes payloads with the execution specification itself and
# supports all post-merge forks. It does not support peer-to-peer
# networking, `/chain.rlp`, or `/blocks/` imports.

set -e

if [ ! -f /genesis.json ]; then
    echo "/genesis.json is missing" >&2
    exit 1
fi

if [ -f /chain.rlp ] || [ -d /blocks ]; then
    echo "eels does not support importing /chain.rlp or /blocks" >&2
    exit 1
fi

echo "Supplied genesis state:"
jq 'del(.alloc)' /genesis.json

echo "Starting EELS engine server..."
cd /execution-specs
exec uv run --no-sync ethereum-spec-engine \
    --genesis /genesis.json \
    --chain-id "${HIVE_CHAIN_ID:-1}" \
    --address 0.0.0.0 \
    --rpc-port 8545 \
    --engine-port 8551
