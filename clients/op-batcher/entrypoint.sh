#!/bin/sh
set -exu

SEQUENCER_BATCH_INBOX_ADDRESS="$(cat /rollup.json | jq -r '.batch_inbox_address')"

exec op-batcher \
    $HIVE_L1_ETH_RPC_FLAG \
    $HIVE_L2_ETH_RPC_FLAG \
    $HIVE_ROLLUP_RPC_FLAG \
    --min-l1-tx-size-bytes=1 \
    --max-l1-tx-size-bytes=120000 \
    --channel-timeout=40 \
    --poll-interval=1s \
    --num-confirmations=1 \
    --safe-abort-nonce-too-low-count=3 \
    --resubmission-timeout=30s \
    --mnemonic="test test test test test test test test test test test junk" \
    --sequencer-hd-path="m/44'/60'/0'/0/2" \
    --sequencer-batch-inbox-address=$SEQUENCER_BATCH_INBOX_ADDRESS \
    --log-terminal=true
