#!/bin/bash
set -e

# Create and start a valid bootnode for the clients to find each other
BOOTNODE_KEYHEX=5ce1f6fc42c817c38939498ee7fcdd9a06fa9d947d02d42dfe814406f370c4eb
BOOTNODE_ENODEID=6433e8fb82c4638a8a6d499d40eb7d8158883219600bfd49acb968e3a37ccced04c964fa87b3a78a2da1b71dc1b90275f4d055720bb67fad4a118a56925125dc

BOOTNODE_IP=`ifconfig eth0 | grep 'inet addr' | cut -d: -f2 | awk '{print $1}'`
BOOTNODE_ENODE="enode://$BOOTNODE_ENODEID@$BOOTNODE_IP:30303"

/bootnode --nodekeyhex $BOOTNODE_KEYHEX --addr=0.0.0.0:30303 &

# Configure the simulation parameters
NO_FORK_NODES=3       # Number of no-fork nodes to boot up
PRO_FORK_NODES=3      # Number of pro-fork nodes to boot up
PRE_FORK_PEERS="0x5"  # Number of peers to require pre-fork (total - 1), RPC reply
POST_FORK_PEERS="0x2" # Number of peers to require post-fork (same camp - 1), RPC reply
DAO_FORK_BLOCK=25     # Block number at which to fork the chain

# Start a batch of clients for the no-fork camp
nofork=()
for i in `seq 1 $NO_FORK_NODES`; do
	echo "Starting no-fork client #$i..."
	nofork+=(`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes`)
done

# Start a batch of clients for the pro-fork camp
profork=()
for i in `seq 1 $PRO_FORK_NODES`; do
	echo "Starting pro-fork client #$i..."
	profork+=(`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO=$DAO_FORK_BLOCK`)
done

allnodes=( "${nofork[@]}" "${profork[@]}" )

# Wait a bit for the nodes to all find each other
sleep 3

# Check that we have a full graph for all nodes
for id in ${allnodes[@]}; do
	ip=`curl -sf $HIVE_SIMULATOR/nodes/$id`
	peers=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":0}' $ip:8545 | jq '.result' | tr -d '"'`

	if [ "$peers" != "$PRE_FORK_PEERS" ]; then
		echo "Invalid peer count for $ip: have $peers, want $PRE_FORK_PEERS"
		exit -1
	fi
done

# Start a miner for each camp, wait until they pass the DAO fork block
noforkMiner=`curl -sf $HIVE_SIMULATOR/nodes/${nofork[0]}`
proforkMiner=`curl -sf $HIVE_SIMULATOR/nodes/${profork[0]}`

curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_start","params":[1],"id":1}' $noforkMiner:8545
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_start","params":[1],"id":1}' $proforkMiner:8545

while [ true ]; do
	block=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}' $noforkMiner:8545 | jq '.result' | tr -d '"'`
	block=$((16#${block#*x}))
	if [ "$block" -gt "$DAO_FORK_BLOCK" ]; then
		break
	fi
	sleep 1
done
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_stop","params":[],"id":3}' $noforkMiner:8545

while [ true ]; do
	block=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}' $proforkMiner:8545 | jq '.result' | tr -d '"'`
	block=$((16#${block#*x}))
	if [ "$block" -gt "$DAO_FORK_BLOCK" ]; then
		break
	fi
	sleep 1
done
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_stop","params":[],"id":3}' $proforkMiner:8545

# Wait a bit for the network to partition
sleep 3

# Check that we have two disjoint est of nodes
for id in ${allnodes[@]}; do
	ip=`curl -sf $HIVE_SIMULATOR/nodes/$id`
	peers=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":4}' $ip:8545 | jq '.result' | tr -d '"'`

	if [ "$peers" != "$POST_FORK_PEERS" ]; then
		echo "Invalid peer count for $ip: have $peers, want $POST_FORK_PEERS"
		exit -1
	fi
done
