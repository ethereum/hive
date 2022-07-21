#!/bin/sh
set -exu

L2OO_ADDRESS=$(jq -r .address < /L2OutputOracleProxy.json)

exec op-proposer \
    $HIVE_L1_ETH_RPC_FLAG \
    $HIVE_L2_ETH_RPC_FLAG \
    $HIVE_ROLLUP_RPC_FLAG \
    --poll-interval 10s \
    --num-confirmations 1 \
    --safe-abort-nonce-too-low-count 3 \
    --resubmission-timeout 30s \
    --mnemonic "test test test test test test test test test test test junk" \
    --l2-output-hd-path "m/44'/60'/0'/0/1" \
    --l2oo-address $L2OO_ADDRESS \
    --log-terminal=true
