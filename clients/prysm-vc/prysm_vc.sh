#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/vc
mkdir -p /data/validators

for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")

  /app/cmd/validator/validator accounts import \
    --prater \
    --wallet-dir="/data/validators" \
    --keys-dir="/hive/input/keystores/$pubkey" \
    --account-password-file="/hive/input/secrets/$pubkey"

done

ls  /data/validators

LOG=info
case "$HIVE_LOGLEVEL" in
    0)   LOG=fatal ;;
    1)   LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

/app/cmd/validator/validator \
    --verbosity="$LOG" \
    --accept-terms-of-use=true \
    --prater \
    --beacon-rpc-provider="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT" \
    --beacon-rpc-gateway-provider="http://$HIVE_ETH2_BN_API_IP:${HIVE_ETH2_BN_GATEWAY_PORT:-3500}" \
    --datadir="/data/vc" \
    --wallet-dir="/data/validators" \
    --chain-config-file="/hive/input/config.yaml"

    # --init-slashing-protection


