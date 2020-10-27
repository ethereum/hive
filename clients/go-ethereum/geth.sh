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
#  - HIVE_NETWORK_ID     network ID number to use for the eth protocol
#  - HIVE_TESTNET        whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE       sync and pruning selector (archive, full, light)
#
# Forks
#
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
#
# Other:
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW       If set, skip PoW verification during block import
#  - HIVE_LOGLEVEL		 Simulator loglevel
#  - HIVE_GRAPHQL_ENABLED      If set, make sure graphql is accessible on port 8550

# Immediately abort the script on any error encountered
set -e

geth=/usr/local/bin/geth
FLAGS="--nousb --pcscdpath=\"\""

# It doesn't make sense to dial out, use only a pre-set bootnode.
FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"

if [ "$HIVE_SKIP_POW" != "" ]; then
	FLAGS="$FLAGS --fakepow"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --networkid $HIVE_NETWORK_ID"
else
    # Unless otherwise specified by hive, we try to avoid mainnet networkid. If geth detects mainnet network id,
    # then it tries to bump memory quite a lot
    FLAGS="$FLAGS --networkid 1337"
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS --testnet"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS --syncmode fast "
fi
if [ "$HIVE_NODETYPE" == "light" ]; then
	FLAGS="$FLAGS --syncmode light "
fi

# Configure the chain.
mv /genesis.json /genesis-input.json
jq -f /mapper.jq /genesis-input.json > /genesis.json

# Dump genesis
echo "Supplied genesis state:"
cat /genesis.json

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
$geth $FLAGS init /genesis.json

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	$geth $FLAGS --gcmode=archive import /chain.rlp
else
	echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	(cd /blocks && $geth $FLAGS --gcmode=archive --verbosity=$HIVE_LOGLEVEL --nocompaction import `ls | sort -n`)
else
	echo "Warning: blocks folder not found."
fi

set -e

# Load any keys explicitly added to the node
if [ -d /keys ]; then
	FLAGS="$FLAGS --keystore /keys"
fi

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --mine --minerthreads 1 --etherbase $HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS --extradata $HIVE_MINER_EXTRA"
fi

# Configure RPC.
FLAGS="$FLAGS --http --http.addr=0.0.0.0 --http.port=8545 --http.api=admin,debug,eth,miner,net,personal,txpool,web3"
FLAGS="$FLAGS --ws --ws.addr=0.0.0.0 --ws.origins \"*\" --ws.api=admin,debug,eth,miner,net,personal,txpool,web3"
if [ "$HIVE_GRAPHQL_ENABLED" != "" ]; then
	FLAGS="$FLAGS --graphql"
fi

# Run the go-ethereum implementation with the requested flags.
FLAGS="$FLAGS --verbosity=$HIVE_LOGLEVEL --nat=none"
echo "Running go-ethereum with flags $FLAGS"
$geth $FLAGS
