#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS --networks=$HIVE_PORTAL_NETWORKS_SELECTED"

    if [[ $HIVE_PORTAL_NETWORKS_SELECTED =~ "beacon" ]]; then
        # Providing a trusted block root is required currently to fully enable the beacon network.
        # It can be a made up value for now as tests are not doing any sync.
        FLAGS="$FLAGS --trusted-block-root:0x0000000000000000000000000000000000000000000000000000000000000000"
    fi
else
    FLAGS="$FLAGS --networks=history"
fi


if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS --netkey-unsafe=0x$HIVE_CLIENT_PRIVATE_KEY"
fi

fluffy --log-level=INFO --rpc --rpc-address="0.0.0.0" --nat:extip:"$IP_ADDR" --portal-network=none \
    --log-level="debug" --disable-state-root-validation $FLAGS
