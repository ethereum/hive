#!/bin/bash

# Immediately abort the script on any error encountered
set -e

FLAGS=""
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

if [ "$HIVE_TRUSTED_BLOCK_ROOT" != "" ]; then
    FLAGS="$FLAGS --trusted-block-root $HIVE_TRUSTED_BLOCK_ROOT"
fi

if [ "$HIVE_BOOTNODES" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODES"
else
    FLAGS="$FLAGS --bootnodes=none"
fi

FLAGS="$FLAGS --nat --disable-init-check extip:$IP_ADDR"

shisui $FLAGS

