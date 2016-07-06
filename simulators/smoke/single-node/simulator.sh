#!/bin/sh
set -e

# Start a new client node
echo -n "Starting a new client..."
id=`curl -s -X POST $HIVE_SIMULATOR/nodes`
echo $id

# Retrieve it's genesis block
ip=`curl -s $HIVE_SIMULATOR/nodes/$id`
curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":0}' $ip:8545

# Terminate the client
curl -s -X DELETE $HIVE_SIMULATOR/nodes/$id
