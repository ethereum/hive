#!/bin/bash

# Immediately abort the script on any error encountered
set -e

FLAGS=""
IP_ADDR=$(hostname -i | awk '{print $1}')

if [ "$HIVE_LOGLEVEL" != "" ]; then
    FLAGS+=" --loglevel $HIVE_LOGLEVEL"
fi

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS+=" --private.key 0x$HIVE_CLIENT_PRIVATE_KEY"
fi

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS+=" --networks $HIVE_PORTAL_NETWORKS_SELECTED"
else
    FLAGS+=" --networks history"
fi

if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS+=" --bootnodes=$HIVE_BOOTNODE"
else
    FLAGS+=" --bootnodes=none"
fi

FLAGS+=" --nat extip:$IP_ADDR"

shisui $FLAGS

