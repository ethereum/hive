#!/bin/bash
set -e

# Create and start a valid bootnode for the clients to find each other
BOOTNODE_KEYHEX=5ce1f6fc42c817c38939498ee7fcdd9a06fa9d947d02d42dfe814406f370c4eb
BOOTNODE_ENODEID=6433e8fb82c4638a8a6d499d40eb7d8158883219600bfd49acb968e3a37ccced04c964fa87b3a78a2da1b71dc1b90275f4d055720bb67fad4a118a56925125dc

BOOTNODE_IP=`ifconfig eth0 | grep 'inet addr' | cut -d: -f2 | awk '{print $1}'`
BOOTNODE_ENODE="enode://$BOOTNODE_ENODEID@$BOOTNODE_IP:30303"

/bootnode --nodekeyhex $BOOTNODE_KEYHEX --addr=0.0.0.0:30303 &

# ethBlockNumber executes an RPC request to a node to retrieve its current block
# number.
#
# If looks up the IP address of the node from the hive simulator and executes the
# actual number lookup. As the result is hex encoded, it will decode and return it.
function ethBlockNumber {
	local block=`curl -sf -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' $1:8545 | jq '.result' | tr -d '"'`
	echo $((16#${block#*x}))
}

# waitBlock waits until the block number of a remote node is at least the specified
# one, periodically polling and sleeping otherwise. There is no timeout, the method
# will block until it succeeds.
function waitBlock {
	local ip=`curl -sf $HIVE_SIMULATOR/nodes/$1`

	while [ true ]; do
		block=`ethBlockNumber $ip`
		if [ "$block" -ge "$2" ]; then
			break
		fi
		sleep 0.5
	done
}

# assertBlock checks that the block number of a remote node is exactly the one
# specified for this method.
function assertBlock {
	local ip=`curl -sf $HIVE_SIMULATOR/nodes/$1`

	local block=`ethBlockNumber $ip`
	if [ "$block" -ne "$2" ]; then
		echo "Block number mismatch for $1: have $block, want $2"
		exit -1
	fi
}

# Start up two pre-seeded Ethereum nodes, one pro-fork another no-fork
cat /genesis.json | jq ". + {\"config\": {\"daoForkBlock\": $HIVE_FORK_DAO_BLOCK}}" > /seed-genesis.json

/geth --datadir=/tmp/profork init /seed-genesis.json
/geth --datadir=/tmp/profork --support-dao-fork import /profork-chain.rlp
/geth --datadir=/tmp/profork --support-dao-fork --nat=none --bootnodes=$BOOTNODE_ENODE --port=30304 &

/geth --datadir=/tmp/nofork init /seed-genesis.json
/geth --datadir=/tmp/nofork --oppose-dao-fork import /nofork-chain.rlp
/geth --datadir=/tmp/nofork --oppose-dao-fork --nat=none --bootnodes=$BOOTNODE_ENODE --port=30305 &

# Start the clients of the on various two forks with different sync mechanisms
profork_arch=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=1\&HIVE_NODETYPE=archive`
profork_full=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=1\&HIVE_NODETYPE=full`

nofork_arch=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=0\&HIVE_NODETYPE=archive`
nofork_full=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=0\&HIVE_NODETYPE=full`

# Wait until all nodes reach their own chains' size
waitBlock $nofork_arch 2048
waitBlock $nofork_full 2048
waitBlock $profork_arch 2176
waitBlock $profork_full 2176

# Sleep a bit and ensure no client passed it's alloted block count
sleep 1

assertBlock $profork_arch 2176
assertBlock $profork_full 2176
assertBlock $nofork_arch 2048
assertBlock $nofork_full 2048
