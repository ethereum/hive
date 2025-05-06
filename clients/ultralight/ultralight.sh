#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS --pk=0x1a2408021220$HIVE_CLIENT_PRIVATE_KEY"
fi

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS --networks=$HIVE_PORTAL_NETWORKS_SELECTED"
else
    FLAGS="$FLAGS --networks=history"
fi

if [ "$HIVE_TRUSTED_BLOCK_ROOT" != "" ]; then
    FLAGS="$FLAGS --trustedBlockRoot $HIVE_TRUSTED_BLOCK_ROOT"
fi

if [ "$HIVE_BOOTNODES" != "" ]; then
    FLAGS="$FLAGS --bootnode=$HIVE_BOOTNODES"
fi

DEBUG=*Portal*,*ultralight* node /ultralight/packages/cli/dist/index.js --bindAddress="$IP_ADDR:9000" --dataDir="./data" --rpcPort=8545 $FLAGS
