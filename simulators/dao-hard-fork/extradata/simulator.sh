#!/bin/bash
set -e

# Start a new client node
echo -n "Starting a new client..."
id=`curl -sf -X POST $HIVE_SIMULATOR/nodes`
ip=`curl -sf $HIVE_SIMULATOR/nodes/$id`
echo "Started node $id at $ip"

# Let it mine past the DAO fork block and the consecutive blocks
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_start","params":[1],"id":0}' $ip:8545

latest=0
while [ true ]; do
	latest=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $ip:8545 | jq '.result' | tr -d '"'`
	latest=$((16#${latest#*x}))
	if [ "$latest" -ge "$((HIVE_FORK_DAO + 13))" ]; then
		break
	fi
	sleep 1
done
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_stop","params":[],"id":2}' $ip:8545

# Retrieve all blocks and validate DAOs' extradata ("dao-hard-fork" -> 0x64616f2d686172642d666f726b)
for block in `seq 0 $latest`; do
	extra=`curl -sf -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[$block, false],\"id\":3}" $ip:8545 | jq '.result.extraData' | tr -d '"'`

	if [ "$block" -ge $HIVE_FORK_DAO ] && [ "$block" -lt $((HIVE_FORK_DAO + 10)) ]; then
		if [ "$extra" != "0x64616f2d686172642d666f726b" ]; then
			echo "DAO fork start (block #$block) extra-data mismatch: have $extra, want 0x64616f2d686172642d666f726b"
			exit -1
		fi
	else
		if [ "$extra" == "0x64616f2d686172642d666f726b" ]; then
			echo "Non DAO fork start (block #$block) has DAO extra-data"
			exit -1
		fi
	fi
done
