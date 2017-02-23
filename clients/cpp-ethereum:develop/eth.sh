#!/bin/bash

# Startup script to initialize and boot a go-ethereum instance.
#
# This script assumes the following files:
#  - `geth` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE       enode URL of the remote bootstrap node
#  - HIVE_TESTNET        whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE       sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_VOTE  whether the node support (or opposes) the DAO fork
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks

# Immediately abort the script on any error encountered
set -e
ETH=cpp-ethereum/build/eth/eth

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --peerset $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --no-discovery"
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS --morden"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "light" ]; then
	FLAGS="$FLAGS --mode peer"
fi

CONFIG="/config_tmp.json"
if [ -f $CONFIG ]; then 
    rm $CONFIG
fi 

chainconfig=`cat /config.json`
if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
	chainconfig=`echo $chainconfig | jq ".params.daoHardforkBlock = \"$HIVE_FORK_DAO_BLOCK\""`
fi
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	chainconfig=`echo $chainconfig | jq ".params.frontierCompatibilityModeLimit = \"$HIVE_FORK_HOMESTEAD\""`
fi
echo $chainconfig > $CONFIG

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	/$ETH $FLAGS import /chain.rlp
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	for block in `ls /blocks | sort -n`; do
		echo /blocks/$block
		/$ETH $FLAGS import /blocks/$block
	done
fi

# Load any keys explicitly added to the node
#if [ -d /keys ]; then
#	FLAGS="$FLAGS --keystore /keys"
#fi

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --mining on --mining-threads 1 --address $HIVE_MINER"
fi
#if [ "$HIVE_MINER_EXTRA" != "" ]; then
#	FLAGS="$FLAGS --extradata $HIVE_MINER_EXTRA"
#fi

# Run the cpp-ethereum implementation with the requested flags
echo "Running cpp-ethereum..."
/$ETH $FLAGS --json-rpc --admin-via-http -v 9
