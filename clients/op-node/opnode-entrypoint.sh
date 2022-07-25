#!/bin/sh
set -exu

# Generate the rollup config.

# Grab the L1 genesis. We can use cURL here to retry.
L1_GENESIS=$(curl \
    --silent \
    --fail \
    --retry 10 \
    --retry-delay 2 \
    --retry-connrefused \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
    $HIVE_L1_URL)

# Grab the L2 genesis. We can use cURL here to retry.
L2_GENESIS=$(curl \
    --silent \
    --fail \
    --retry 10 \
    --retry-delay 2 \
    --retry-connrefused \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
    $HIVE_L2_URL)

DEPOSIT_CONTRACT_ADDRESS=$(jq -r .address < /OptimismPortalProxy.json)

jq ". | .genesis.l1.hash = \"$(echo $L1_GENESIS | jq -r '.result.hash')\"" < /rollup.json | \
   jq ". | .genesis.l2.hash = \"$(echo $L2_GENESIS | jq -r '.result.hash')\"" | \
   jq ". | .genesis.l2_time = $(echo $L2_GENESIS | jq -r '.result.timestamp' | xargs printf "%d")" | \
   jq ". | .deposit_contract_address = \"$DEPOSIT_CONTRACT_ADDRESS\"" > /hive/rollup.json

exec op-node \
    $HIVE_L1_ETH_RPC_FLAG \
    $HIVE_L2_ENGINE_RPC_FLAG \
    --l2.jwt-secret=/config/test-jwt-secret.txt \
    $HIVE_SEQUENCER_ENABLED_FLAG \
    $HIVE_SEQUENCER_KEY_FLAG \
    --sequencer.l1-confs=0 \
    --verifier.l1-confs=0 \
    --rollup.config=/hive/rollup.json \
    --rpc.addr=0.0.0.0 \
    --rpc.port=7545 \
    --p2p.listen.ip=0.0.0.0 \
    --p2p.listen.tcp=9003 \
    --p2p.listen.udp=9003 \
    $HIVE_P2P_STATIC_FLAG \
    --snapshotlog.file=/snapshot.log \
    --p2p.priv.path=/config/p2p-node-key.txt
