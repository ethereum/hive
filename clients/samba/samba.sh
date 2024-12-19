#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS  --unsafe-private-key=0x$HIVE_CLIENT_PRIVATE_KEY"
fi

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS  --portal-subnetworks=$HIVE_PORTAL_NETWORKS_SELECTED"
else
    FLAGS="$FLAGS  --portal-subnetworks=history"
fi

/opt/samba/bin/samba \
    --jsonrpc-port=8545 \
    --jsonrpc-host="${IP_ADDR}" \
    --p2p-ip="${IP_ADDR}" \
    --use-default-bootnodes=false \
    $FLAGS

