#!/bin/sh
set -exu

# Generate the rollup config.

L1_URL="http://172.17.0.3:8545"
L2_URL="http://172.17.0.4:9545"

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
    $L1_URL)

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
    $L2_URL)

DEPOSIT_CONTRACT_ADDRESS=$(jq -r .address < /OptimismPortal.json)

jq ". | .genesis.l1.hash = \"$(echo $L1_GENESIS | jq -r '.result.hash')\"" < /rollup.json | \
   jq ". | .genesis.l2.hash = \"$(echo $L2_GENESIS | jq -r '.result.hash')\"" | \
   jq ". | .genesis.l2_time = $(echo $L2_GENESIS | jq -r '.result.timestamp' | xargs printf "%d")" | \
   jq ". | .deposit_contract_address = \"$DEPOSIT_CONTRACT_ADDRESS\"" > /hive/rollup.json

exec op-node \
    --l1=ws://172.17.0.3:8546 \
    --l2=ws://172.17.0.4:9546 \
    --sequencing.enabled \
    --p2p.sequencer.key=/config/p2p-sequencer-key.txt \
    --rollup.config=/hive/rollup.json \
    --l2.eth=http://172.17.0.4:9545 \
    --rpc.addr=0.0.0.0 \
    --rpc.port=7545 \
    --p2p.listen.ip=0.0.0.0 \
    --p2p.listen.tcp=9003 \
    --p2p.listen.udp=9003 \
    --snapshotlog.file=/snapshot.log \
    --p2p.priv.path=/config/p2p-node-key.txt
