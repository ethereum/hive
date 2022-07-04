#!/bin/sh
set -exu

# Generate the bss config.

L2_URL="http://172.17.0.4:9545"

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

SEQUENCER_GENESIS_HASH="$(echo $L2_GENESIS | jq -r '.result.hash')"
SEQUENCER_BATCH_INBOX_ADDRESS="$(cat /rollup.json | jq -r '.batch_inbox_address')"

exec op-batcher \
    --l1-eth-rpc=http://172.17.0.3:8545 \
    --l2-eth-rpc=http://172.17.0.4:9545 \
    --rollup-rpc=http://172.17.0.5:7545 \
    --min-l1-tx-size-bytes=1 \
    --max-l1-tx-size-bytes=120000 \
    --poll-interval=1s \
    --num-confirmations=1 \
    --safe-abort-nonce-too-low-count=3 \
    --resubmission-timeout=30s \
    --mnemonic="test test test test test test test test test test test junk" \
    --sequencer-hd-path="m/44'/60'/0'/0/2" \
    --sequencer-history-db-filename="history_db.json" \
    --sequencer-batch-inbox-address=$SEQUENCER_BATCH_INBOX_ADDRESS \
    --log-terminal=true
