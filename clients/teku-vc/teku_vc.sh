#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/teku
mkdir -p /data/validators/keys
mkdir -p /data/validators/passwords


for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")
  cp "/hive/input/keystores/$pubkey/keystore.json" "/data/validators/keys/voting-keystore-$pubkey.json"
  cp "/hive/input/secrets/$pubkey" "/data/validators/passwords/voting-keystore-$pubkey.txt"
done

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0|1) LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

/opt/teku/bin/teku vc \
    --network=auto \
    --beacon-node-api-endpoint="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT" \
    --data-path=/data/teku \
    --logging="$LOG" \
    --validator-keys=/data/validators/keys:/data/validators/passwords \
    --validators-proposer-default-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" \
    --validators-external-signer-slashing-protection-enabled=true
