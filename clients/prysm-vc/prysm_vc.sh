#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/vc
mkdir -p /data/validators
echo walletpassword > /wallet.pass

for keystore_path in /hive/input/keystores/*
do
  pubkey=$(basename "$keystore_path")

  /validator accounts import \
    --prater \
    --accept-terms-of-use=true \
    --wallet-dir="/data/validators" \
    --keys-dir="/hive/input/keystores/$pubkey" \
    --account-password-file="/hive/input/secrets/$pubkey" \
    --wallet-password-file="/wallet.pass"

done

ls  /data/validators

# znrt encodes the values of these items between double quotes (""), which is against the spec:
# https://github.com/ethereum/consensus-specs/blob/v1.1.10/configs/mainnet.yaml
sed -i 's/"\([[:digit:]]\+\)"/\1/' /hive/input/config.yaml
sed -i 's/"\(0x[[:xdigit:]]\+\)"/\1/' /hive/input/config.yaml

echo config.yaml:
cat /hive/input/config.yaml

LOG=info
case "$HIVE_LOGLEVEL" in
    0)   LOG=fatal ;;
    1)   LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--enable-builder --suggested-fee-recipient=0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b")
echo BUILDER=$builder_option

echo Starting Prysm Validator Client

/validator \
    --verbosity="$LOG" \
    --accept-terms-of-use=true \
    --prater \
    --beacon-rpc-provider="$HIVE_ETH2_BN_API_IP:${HIVE_ETH2_BN_GRPC_PORT:-3500}" \
    --beacon-rpc-gateway-provider="$HIVE_ETH2_BN_API_IP:${HIVE_ETH2_BN_API_PORT:-4000}" \
    --datadir="/data/vc" \
    --wallet-dir="/data/validators" \
    --wallet-password-file="/wallet.pass" \
    --chain-config-file="/hive/input/config.yaml" \
    $builder_option
# NOTE: gRPC/RPC ports are inverted to allow the simulator to access the REST API