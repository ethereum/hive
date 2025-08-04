#!/bin/bash

# Immediately abort the script on any error encountered
set -e

mkdir -p /data/vc

# Nimbus wants all secrets with 0600 permissions
chmod 0600 /hive/input/secrets/*

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0|1) LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--payload-builder=true")
echo BUILDER=$builder_option

echo Starting Nimbus Validator Client

/usr/bin/nimbus_validator_client \
    --non-interactive=true \
    --log-level="$LOG" \
    --data-dir=/data/vc \
    --beacon-node="http://$HIVE_ETH2_BN_API_IP:$HIVE_ETH2_BN_API_PORT" \
    --validators-dir="/hive/input/keystores" \
    --secrets-dir="/hive/input/secrets" \
    $builder_option
