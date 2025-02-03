#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_PORTAL_NETWORKS_SELECTED" != "" ]; then
    FLAGS="$FLAGS --portal-subnetworks=$HIVE_PORTAL_NETWORKS_SELECTED"

    if [[ $HIVE_PORTAL_NETWORKS_SELECTED =~ "beacon" ]]; then
        # Providing a trusted block root is required currently to fully enable the beacon network.
        if [ "$HIVE_TRUSTED_BLOCK_ROOT" != "" ]; then
            # A trusted block root is provided for tests that are doing sync.
            FLAGS="$FLAGS --trusted-block-root:$HIVE_TRUSTED_BLOCK_ROOT"
        else
            # It can be a made up value for tests that are not doing any sync.
            FLAGS="$FLAGS --trusted-block-root:0x0000000000000000000000000000000000000000000000000000000000000000"
        fi
    fi
else
    FLAGS="$FLAGS --portal-subnetworks=history"
fi

if [ "$HIVE_BOOTNODES" != "" ]; then
    FLAGS="$FLAGS --bootstrap-node=$HIVE_BOOTNODES"
else
    FLAGS="$FLAGS --network=none"
fi

if [ "$HIVE_CLIENT_PRIVATE_KEY" != "" ]; then
    FLAGS="$FLAGS --netkey-unsafe=0x$HIVE_CLIENT_PRIVATE_KEY"
fi

fluffy --log-level=INFO --rpc --rpc-address="0.0.0.0" --rpc-api="eth,portal,discovery" --nat:extip:"$IP_ADDR" --log-level="debug" \
	--disable-state-root-validation $FLAGS
