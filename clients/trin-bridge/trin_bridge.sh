#!/bin/bash

# Immediately abort the script on any error encountered
set -e

IP_ADDR=$(hostname -i | awk '{print $1}')
FLAGS=""

if [ "$HIVE_BOOTNODES" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODES"
else
    echo "Warning: HIVE_BOOTNODES wasn't provided"
    exit 1
fi

RUST_LOG="trace,neli=error,rustls=error,polling=error,async_io=error,hyper=error,discv5=info,jsonrpsee_types::params=error,jsonrpsee_core::http_helpers=error" portal-bridge $FLAGS --executable-path /usr/bin/trin --mode test:/test_data_collection_of_forks_blocks.yaml --external-ip $IP_ADDR --epoch-accumulator-path .
