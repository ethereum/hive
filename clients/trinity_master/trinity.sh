#!/bin/bash

# Startup script 



# Immediately abort the script on any error encountered
set -e

if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --preferred-node $HIVE_BOOTNODE"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
else
	#default it to ropsten
	FLAGS="$FLAGS --network-id 3"
fi

# Configure and set the chain definition for the node
#chainconfig=`cat /config.json`
configoverride=`jq -f /mapper.jq /genesis.json`

echo ".*$configoverride">/tempscript.jq

mergedconfig=`jq -f /tempscript.jq /config.json`

echo $mergedconfig>/newconfig.json

mkdir /dd
mkdir /dd/logs
cd /
#we need something to indicate to Hive that the client container is alive
#nc –l 8545 &

(sleep 2 ; python -m http.server 8545 )&

# Run the client with the requested flags
echo "Running Trinity... "

RUNCMD="trinity --data-dir /dd $FLAGS --genesis /newconfig.json"
echo "cmd: $RUNCMD"
$RUNCMD

