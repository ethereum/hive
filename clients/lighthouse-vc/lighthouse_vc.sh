#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/testnet_setup
mkdir -p /data/vc
mkdir -p /data/validators

cp /hive/input/config.yaml /data/testnet_setup

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" > /data/testnet_setup/deploy_block.txt

for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")
  mkdir "/data/validators/$pubkey"
  cp "/hive/input/keystores/$pubkey/keystore.json" "/data/validators/$pubkey/voting-keystore.json"
done

cp -r /hive/input/secrets /data/secrets

LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--builder-proposals")

lighthouse \
    --debug-level="$LOG" \
    --datadir=/data/vc \
    --testnet-dir=/data/testnet_setup \
    validator \
    --validators-dir="/data/validators" \
    --secrets-dir="/data/secrets" \
    --init-slashing-protection \
    --beacon-nodes="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT" \
    --suggested-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" \
    $builder_option
