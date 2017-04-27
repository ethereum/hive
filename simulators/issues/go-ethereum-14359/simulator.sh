#!/bin/bash
set -e

# netPeerCount executes an RPC request to a node to retrieve its current number
# of connected peers.
#
# If looks up the IP address of the node from the hive simulator and executes the
# actual peer lookup. As the result is hex encoded, it will decode and return it.
function netPeerCount {
	local peers=`curl -sf -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"net_peerCount","params":[],"id":0}' $1:8545 | jq '.result' | tr -d '"'`
	echo $((16#${peers#*x}))
}

# waitPeers verifies whether a node has a given number of peers, and if not waits
# a bit and retries a few times (to avoid devp2p pending handshakes and similar
# anomalies). If the peer count doesn't converge to the desired target, the method
# will fail.
function waitPeers {
	local ip=`curl -sf $HIVE_SIMULATOR/nodes/$1`

	for i in `seq 1 10`; do
		# Fetch the current peer count and stop if it's correct
		peers=`netPeerCount $ip`
		if [ "$peers" == "$2" ]; then
			break
		fi
		# Seems peer count is wrong, unless too many trials, sleep a bit and retry
		if [ "$i" == "10" ]; then
			echo "Invalid peer count for $1 ($ip): have $peers, want $2"
			exit -1
		fi

		sleep 0.5
	done
}

# Create and start a valid bootnode for the clients to find each other
BOOTNODE_KEYHEX=5ce1f6fc42c817c38939498ee7fcdd9a06fa9d947d02d42dfe814406f370c4eb
BOOTNODE_ENODEID=6433e8fb82c4638a8a6d499d40eb7d8158883219600bfd49acb968e3a37ccced04c964fa87b3a78a2da1b71dc1b90275f4d055720bb67fad4a118a56925125dc

BOOTNODE_IP=`ifconfig eth0 | grep 'inet addr' | cut -d: -f2 | awk '{print $1}'`
BOOTNODE_ENODE="enode://$BOOTNODE_ENODEID@$BOOTNODE_IP:30303"

/bootnode --nodekeyhex $BOOTNODE_KEYHEX --addr=0.0.0.0:30303 &

# Start two simple nodes an interconnect them via the bootnode
nodeA=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_NETWORK_ID=48314142015`
sleep 1

nodeB=`curl -sf -X POST --data-urlencode "HIVE_BOOTNODE=$BOOTNODE_ENODE" $HIVE_SIMULATOR/nodes?HIVE_NETWORK_ID=48314142015`
sleep 1

# Ensure both nodes connect to each other
waitPeers $nodeA 1
waitPeers $nodeB 1
