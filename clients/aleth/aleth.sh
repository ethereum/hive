#!/bin/bash

# Startup script to initialize and boot an Aleth instance.
#
# This script assumes the following files:
#  - `eth` binary is located somewhere on the $PATH
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#
#  - HIVE_BOOTNODE       enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID     network ID number to use for the eth protocol
#  - HIVE_TESTNET        whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE       sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_VOTE  whether the node support (or opposes) the DAO fork
#  - HIVE_FORK_TANGERINE block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS  block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG  block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_FORK_ISTANBUL block number for Istanbul
#  - HIVE_FORK_MUIRGLACIER block number for Muir Glacier
#  - HIVE_FORK_BERLIN block number for Berlin
#  - HIVE_FORK_LONDON block number for London
#
# Other:
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW       If set, skip PoW verification during block import
#  - HIVE_LOGLEVEL		 Simulator loglevel

# Immediately abort the script on any error encountered
set -e

# Log level.
LOG=2
case "$HIVE_LOGLEVEL" in
	0|1) LOG=1 ;; # error
	2|3) LOG=2 ;; # info (default)
	4)   LOG=3 ;; # debug
	5)   LOG=4 ;; # trace
esac
FLAGS="-v $LOG"

# Configure the chain.
# See http://www.ethdocs.org/en/latest/network/test-networks.html#custom-networks-eth
# for info about how it's configured
jq -f /mapper.jq /genesis.json > /config.json
echo -n "Chain config: "; cat /config.json

FLAGS="$FLAGS --config /config.json"
ETHEXEC=/usr/bin/aleth
echo "Flags: $FLAGS"

# Don't immediately abort, some imports are meant to fail
set +e

if [ -f /chain.rlp ]; then
	echo "Loading chain.rlp..."
	echo "Command: $ETHEXEC $FLAGS --import /chain.rlp"
	$ETHEXEC $FLAGS --import /chain.rlp
fi

# Load the remainder of the test chain
if [ -d /blocks ]; then
	echo "Loading remaining individual blocks..."
	for block in `ls /blocks | sort -n`; do
		echo "Command: aleth $FLAGS --import /blocks/$block"
		$ETHEXEC $FLAGS --import /blocks/$block
		#valgrind --leak-check=yes $ETHEXEC $FLAGS import /blocks/$block
		#gdb -q -n -ex r -ex bt --args eth $FLAGS import /blocks/$block
	done
fi

# Configure peer-to-peer networking.
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --peerset required:$HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --no-bootstrap"
fi
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
fi

# Configure mining
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --mining --address $HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS --extradata $HIVE_MINER_EXTRA"
fi

echo "Running Aleth..."
# FLAGS="$FLAGS --allow-local-discovery"
RUNCMD="python3 /usr/bin/aleth.py --rpc http://0.0.0.0:8545 --aleth-exec $ETHEXEC $FLAGS"
echo "cmd: $RUNCMD"
$RUNCMD
