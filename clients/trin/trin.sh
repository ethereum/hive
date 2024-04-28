#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS --unsafe-private-key 0x$HIVE_CLIENT_PRIVATE_KEY"
fi

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS --portal-subnetworks $HIVE_PORTAL_NETWORKS_SELECTED"
else
    FLAGS="$FLAGS --portal-subnetworks history"
fi

RUST_LOG="trace,neli=error" trin --web3-transport http --web3-http-address http://0.0.0.0:8545 --external-address "$IP_ADDR":9009 --bootnodes none $FLAGS
