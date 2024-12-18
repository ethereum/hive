#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS  --unsafe-private-key=0x$HIVE_CLIENT_PRIVATE_KEY"
fi

 echo $HIVE_CLIENT_PRIVATE_KEY

/opt/samba/bin/samba \
    --jsonrpc-port=8545 \
    --jsonrpc-host="${IP_ADDR}" \
    --p2p-ip="${IP_ADDR}" \
    --use-default-bootnodes=false \
    --portal-subnetworks="history-network" \
    $FLAGS