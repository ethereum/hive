#!/bin/sh
set -exu

curl \
    --fail \
    --retry 10 \
    --retry-delay 2 \
    --retry-connrefused \
    -X POST \
    -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' \
    http://172.17.0.3:8545

L2OO_ADDRESS=$(jq -r .address < /L2OutputOracle.json)

exec l2os \
    --l1-eth-rpc http://172.17.0.3:8545 \
    --l2-eth-rpc http://172.17.0.4:9545 \
    --rollup-rpc http://172.17.0.5:7545 \
    --l2oo-address $L2OO_ADDRESS \
    --poll-interval 10s \
    --num-confirmations 1 \
    --safe-abort-nonce-too-low-count 3 \
    --resubmission-timeout 30s \
    --mnemonic "test test test test test test test test test test test junk" \
    --l2-output-hd-path "m/44'/60'/0'/0/1"
