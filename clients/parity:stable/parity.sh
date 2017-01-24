#!/bin/bash

# Startup script to initialize and boot a parity instance.
#
# This script assumes the following files:
#  - `parity` binary is located in the filesystem root
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

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --nodiscover"
fi

# Configure and set the chain definition for the node
chainconfig=`cat /chain.json`

genesis=`cat /genesis.json`
genesis="${genesis/coinbase/author}"

nonce=`echo $genesis | jq ".nonce"` && genesis=`echo $genesis | jq "del(.nonce)"`
mixhash=`echo $genesis | jq ".mixHash"` && genesis=`echo $genesis | jq "del(.mixHash)"`
accounts=`echo $genesis | jq ".alloc"` && genesis=`echo $genesis | jq "del(.alloc)"`
genesis=`echo $genesis | jq ". + {\"seal\": {\"ethereum\": {\"nonce\": $nonce, \"mixHash\": $mixhash}}}"`

if [ "$accounts" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". * {\"accounts\": $accounts}"`
fi
chainconfig=`echo $chainconfig | jq ". + {\"genesis\": $genesis}"`

if [ "$HIVE_TESTNET" == "1" ]; then
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"accountStartNonce\"]; \"0x0100000\")"`
	for account in `echo $chainconfig | jq '.accounts | keys[]'`; do
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", $account, \"nonce\"]; \"0x0100000\")"`
	done
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"frontierCompatibilityModeLimit\"]; \"0x789b0\")"`
fi
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	HIVE_FORK_HOMESTEAD=`echo "obase=16; $HIVE_FORK_HOMESTEAD" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"frontierCompatibilityModeLimit\"]; \"0x$HIVE_FORK_HOMESTEAD\")"`
fi

echo $chainconfig > /chain.json
FLAGS="$FLAGS --chain /chain.json"

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	/parity $FLAGS import /chain.rlp
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	for block in `ls /blocks | sort -n`; do
		/parity $FLAGS import /blocks/$block
	done
fi

# Load any keys explicitly added to the node
if [ -d /keys ]; then
	FLAGS="$FLAGS --keys-path /keys"
fi

# If mining was requested, fire up an ethminer instance
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --author $HIVE_MINER"
	ethminer --mining-threads 1 &
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS --extra-data $HIVE_MINER_EXTRA"
fi

# Run the parity implementation with the requested flags
echo "Running parity..."
/parity $FLAGS --usd-per-eth 1 --nat none --jsonrpc-interface all # --jsonrpc-hosts all <- not available on stable yet
