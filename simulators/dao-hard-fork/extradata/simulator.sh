#!/bin/bash
set -e

# Start two DAO enabled clients, one pro and one contra fork (don't connect them)
echo -n "Starting DAO enabled clients..."

proID=`curl -sf -X POST $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=1\&HIVE_MINER=0x00000000000000000000000000000000000001`
proIP=`curl -sf $HIVE_SIMULATOR/nodes/$proID`
echo "Started pro-fork node $proID at $proIP"

conID=`curl -sf -X POST $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=0\&HIVE_MINER=0x00000000000000000000000000000000000001\&HIVE_MINER_EXTRA=dao-hard-fork`
conIP=`curl -sf $HIVE_SIMULATOR/nodes/$conID`
echo "Started contra-fork node $conID at $conIP"

# Let both nodes mine past the DAO fork blocks by a few
proLatest=0
while [ true ]; do
	proLatest=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $proIP:8545 | jq '.result' | tr -d '"'`
	proLatest=$((16#${proLatest#*x}))
	if [ "$proLatest" -ge "$((HIVE_FORK_DAO_BLOCK + 13))" ]; then
		break
	fi
	sleep 0.25
done

conLatest=0
while [ true ]; do
	conLatest=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $conIP:8545 | jq '.result' | tr -d '"'`
	conLatest=$((16#${conLatest#*x}))
	if [ "$conLatest" -ge "$((HIVE_FORK_DAO_BLOCK + 13))" ]; then
		break
	fi
	sleep 0.25
done

# Retrieve all pro-fork blocks and validate DAO fork range extradata ("dao-hard-fork" -> 0x64616f2d686172642d666f726b)
for block in `seq 1 $proLatest`; do
	extra=`curl -sf -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[$block, false],\"id\":3}" $proIP:8545 | jq '.result.extraData' | tr -d '"'`

	if [ "$block" -ge $HIVE_FORK_DAO_BLOCK ] && [ "$block" -lt $((HIVE_FORK_DAO_BLOCK + 10)) ]; then
		if [ "$extra" != "0x64616f2d686172642d666f726b" ]; then
			echo "[pro] DAO fork start (block #$block) extra-data mismatch: have $extra, want 0x64616f2d686172642d666f726b"
			exit -1
		fi
	else
		if [ "$extra" == "0x64616f2d686172642d666f726b" ]; then
			echo "[pro] Non DAO fork start (block #$block) has DAO extra-data"
			exit -1
		fi
	fi
done

# Retrieve all contra-fork blocks and validate DAO fork range extradata (NOT "dao-hard-fork" -> 0x64616f2d686172642d666f726b)
for block in `seq 1 $conLatest`; do
	extra=`curl -sf -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBlockByNumber\",\"params\":[$block, false],\"id\":3}" $conIP:8545 | jq '.result.extraData' | tr -d '"'`

	if [ "$block" -ge $HIVE_FORK_DAO_BLOCK ] && [ "$block" -lt $((HIVE_FORK_DAO_BLOCK + 10)) ]; then
		if [ "$extra" == "0x64616f2d686172642d666f726b" ]; then
			echo "[con] DAO fork start (block #$block) has DAO extra-data"
			exit -1
		fi
	else
		if [ "$extra" != "0x64616f2d686172642d666f726b" ]; then
			echo "[con] Non DAO fork start (block #$block) extra-data mismatch: have $extra, want 0x64616f2d686172642d666f726b"
			exit -1
		fi
	fi
done
