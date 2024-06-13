#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')

if [ "$HIVE_LOGLEVEL" != "" ]; then
    FLAGS="$FLAGS --loglevel $HIVE_LOGLEVEL"
fi

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS --private.key 0x$HIVE_CLIENT_PRIVATE_KEY"
fi

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS --networks $HIVE_PORTAL_NETWORKS_SELECTED"
else
    FLAGS="$FLAGS --networks history"
fi

if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi

FLAGS="$FLAGS --nat extip:$IP_ADDR"

app $FLAGS

