#!/bin/sh

# Startup script to initialize and boot a go-ethereum instance.
#
# This script assumes the following files:
#  - `geth` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root
#  - `chain.rlp` file is located in the filesystem root
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE enode URL of the remote bootstrap node
#  - HIVE_TESTNET  whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE sync and pruning selector (archive, full, light)

# Immediately abort the script on any error encountered
set -e

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --bootnodes \"\""
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS --testnet"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS --fast"
fi
if [ "$HIVE_NODETYPE" == "light" ]; then
	FLAGS="$FLAGS --light"
fi

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
/geth $FLAGS init /genesis.json
echo

# Load the test chain if present
echo "Loading initial blockchain..."
/geth $FLAGS import /chain.rlp
echo

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
for block in `ls /blocks | sort -n`; do
	/geth $FLAGS import /blocks/$block
done
echo

# Run the go-ethereum implementation with the requested flags
echo "Running go-ethereum..."
/geth $FLAGS --nat=none --rpc --rpcaddr "0.0.0.0" --rpcapi "admin,debug,eth,miner,net,personal,shh,txpool,web3" --etherbase "0x00000000000000000000000000000000000001"
echo
