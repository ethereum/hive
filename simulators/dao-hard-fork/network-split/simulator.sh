#!/bin/bash
set -e

# Create and start a valid bootnode for the clients to find each other
BOOTNODE_KEYHEX=5ce1f6fc42c817c38939498ee7fcdd9a06fa9d947d02d42dfe814406f370c4eb
BOOTNODE_ENODEID=6433e8fb82c4638a8a6d499d40eb7d8158883219600bfd49acb968e3a37ccced04c964fa87b3a78a2da1b71dc1b90275f4d055720bb67fad4a118a56925125dc

BOOTNODE_IP=`ifconfig eth0 | grep 'inet addr' | cut -d: -f2 | awk '{print $1}'`
BOOTNODE_ENODE="enode://$BOOTNODE_ENODEID@$BOOTNODE_IP:30303"

/bootnode --nodekeyhex $BOOTNODE_KEYHEX --addr=0.0.0.0:30303 &

# Configure the simulation parameters
NO_FORK_NODES=3   # Number of no-fork nodes to boot up
PRO_FORK_NODES=3  # Number of pro-fork nodes to boot up
PRE_FORK_PEERS=5  # Number of peers to require pre-fork (total - 1), RPC reply
POST_FORK_PEERS=2 # Number of peers to require post-fork (same camp - 1), RPC reply

# netPeerCount executes an RPC request to a node to retrieve its current number
# of connected peers.
#
# If looks up the IP address of the node from the hive simulator and executes the
# actual peer lookup. As the result is hex encoded, it will decode and return it.
function netPeerCount {
	local ip=`curl -sf $HIVE_SIMULATOR/nodes/$1`
	local peers=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":0}' $ip:8545 | jq '.result' | tr -d '"'`

	echo $((16#${peers#*x}))
}

# Start a batch of clients for the no-fork camp
nofork=()
for i in `seq 1 $NO_FORK_NODES`; do
	echo "Starting no-fork client #$i..."
	nofork+=(`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=0`)
	sleep 1 # Wait a bit until it's registered by the bootnode
done

# Start a batch of clients for the pro-fork camp
profork=()
for i in `seq 1 $PRO_FORK_NODES`; do
	echo "Starting pro-fork client #$i..."
	profork+=(`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_FORK_DAO_VOTE=1`)
	sleep 1 # Wait a bit until it's registered by the bootnode
done

allnodes=( "${nofork[@]}" "${profork[@]}" )

# Wait a bit for the nodes to all find each other
sleep 3

# Check that we have a full graph for all nodes
for id in ${allnodes[@]}; do
	peers=`netPeerCount $id`
	if [ "$peers" != "$PRE_FORK_PEERS" ]; then
		echo "Invalid peer count for $id: have $peers, want $PRE_FORK_PEERS"
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
	if [ "$block" -gt "$((HIVE_FORK_DAO_BLOCK + 10))" ]; then
		break
	fi
	sleep 1
done
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_stop","params":[],"id":3}' $noforkMiner:8545

while [ true ]; do
	block=`curl -sf -X POST --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":2}' $proforkMiner:8545 | jq '.result' | tr -d '"'`
	block=$((16#${block#*x}))
	if [ "$block" -gt "$((HIVE_FORK_DAO_BLOCK + 10))" ]; then
		break
	fi
	sleep 1
done
curl -sf -X POST --data '{"jsonrpc":"2.0","method":"miner_stop","params":[],"id":3}' $proforkMiner:8545

# Check that we have two disjoint set of nodes
for id in ${allnodes[@]}; do
	# devp2p is a bit feisty, so retry a few times to ensure pending handshakes don't count
	for i in `seq 1 3`; do
		# Fetch the current peer count and stop if it's correct
		peers=`netPeerCount $id`
		if [ "$peers" == "$POST_FORK_PEERS" ]; then
			break
		fi
		ip=`curl -sf $HIVE_SIMULATOR/nodes/$id`
		curl -sf -X POST --data '{"jsonrpc":"2.0","method":"admin_peers","params":[],"id":0}' $ip:8545

		# Seems peer count is wrong, unless too many trials, sleep a bit and retry
		if [ "$i" == "3" ]; then
			echo "Invalid peer count for $id: have $peers, want $POST_FORK_PEERS"
			exit -1
		fi

		sleep 0.5
	done
done
