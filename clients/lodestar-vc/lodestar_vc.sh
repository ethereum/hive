#!/bin/sh

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/testnet_setup
mkdir -p /data/vc
mkdir -p /data/validators
mkdir -p /data/secrets

cp /hive/input/config.yaml /data/testnet_setup

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" > /data/testnet_setup/deploy_block.txt

for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")
  mkdir "/data/validators/$pubkey"
  cp "/hive/input/keystores/$pubkey/keystore.json" "/data/validators/$pubkey/voting-keystore.json"
  cp "/hive/input/secrets/$pubkey" "/data/secrets/$pubkey"
done

metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics.address=$CONTAINER_IP --metrics.port=$HIVE_ETH2_METRICS_PORT")

LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--builder --suggestedFeeRecipient 0xa94f5374Fce5edBC8E2a8697C15331677e6EbF0B")
echo BUILDER=$builder_option

echo Starting Lodestar Validator Client

node /usr/app/node_modules/.bin/lodestar \
    validator \
    --logLevel="$LOG" \
    --dataDir=/data/vc \
    --paramsFile=/hive/input/config.yaml \
    --keystoresDir="/data/validators" \
    --secretsDir="/data/secrets" \
    $metrics_option $builder_option \
    --beaconNodes="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT"

