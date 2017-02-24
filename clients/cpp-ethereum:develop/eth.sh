#!/bin/bash

# Startup script to initialize and boot a cpp-ethereum instance.
#
# This script assumes the following files:
#  - `eth` binary is located somewhere on the $PATH
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
#  - HIVE_FORK_TANGERINE block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS  block number of SpurioisDragon
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks

# Immediately abort the script on any error encountered
set -e

# See http://www.ethdocs.org/en/latest/network/test-networks.html#custom-networks-eth
# for info about how it's configured

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --no-discovery"
fi

# Configure and set the chain definition for the node
chainconfig=`cat /config.json`
genesis=`cat /genesis.json`
genesis="${genesis/coinbase/author}"

accounts=`echo $genesis | jq ".alloc"` && genesis=`echo $genesis | jq "del(.alloc)"`

if [ "$accounts" != "" ]; then
	#In some cases, the 'alloc' portion can be extremely large
	# Because of this, it can't be handled via cmd line parameters, 
	# This fails :
	# chainconfig=`echo $chainconfig | jq ". * {\"accounts\": $accounts}"`
	# The following solution instead uses two temporary files

	echo $accounts| jq "{ \"accounts\": .}" > tmp1
	echo $chainconfig > tmp2
	chainconfig=`jq -s '.[0] * .[1]' tmp1 tmp2`
fi
chainconfig=`echo $chainconfig | jq ". + {\"genesis\": $genesis}"`

# See https://github.com/ethcore/parity/wiki/Consensus-Engines for info about options

if [ "$HIVE_TESTNET" == "1" ]; then
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"accountStartNonce\"]; \"0x0100000\")"`
	for account in `echo $chainconfig | jq '.accounts | keys[]'`; do
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", $account, \"nonce\"]; \"0x0100000\")"`
	done
fi
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	HIVE_FORK_HOMESTEAD=`echo "obase=16; $HIVE_FORK_HOMESTEAD" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"homsteadForkBlock\"]; \"0x$HIVE_FORK_HOMESTEAD\")"`
fi

if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
	HIVE_FORK_DAO_BLOCK=`echo "obase=16; $HIVE_FORK_DAO_BLOCK" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"daoHardforkBlock\"]; \"0x$HIVE_FORK_DAO_BLOCK\")"`
fi

if [ "$HIVE_FORK_TANGERINE" != "" ]; then
	HIVE_FORK_TANGERINE=`echo "obase=16; $HIVE_FORK_TANGERINE" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"EIP150ForkBlock\"]; \"0x$HIVE_FORK_TANGERINE\" )"`
fi

if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
	HIVE_FORK_SPURIOUS=`echo "obase=16; $HIVE_FORK_SPURIOUS" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"EIP158ForkBlock\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"EIP158ForkBlock\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"EIP158ForkBlock\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"EIP158ForkBlock\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
fi


echo $chainconfig > /chain2.json

# Initialize the local testchain with the genesis state
FLAGS="$FLAGS --config /chain2.json"
ETHEXEC=/usr/local/bin/eth
echo "Flags: $FLAGS"
#echo "Initializing database with genesis state and loading inital blockchain"
#if [ -f /chain.rlp ]; then
#	eth $FLAGS import /chain.rlp
#fi

# Don't immediately abort, some imports are meant to fail
set +e

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	for block in `ls /blocks | sort -n`; do
		echo "Command: eth $FLAGS import /blocks/$block"
		$ETHEXEC $FLAGS import /blocks/$block
		#valgrind --leak-check=yes $ETHEXEC $FLAGS import /blocks/$block
		#gdb -q -n -ex r -ex bt --args eth $FLAGS import /blocks/$block		
	done
fi

# Load any keys explicitly added to the node
echo "Not importing keys, TODO implement relevant command"

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --mining --address $HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS --extradata $HIVE_MINER_EXTRA"
fi

# Run the go-ethereum implementation with the requested flags
echo "Running cpp-ethereum..."
eth $FLAGS --json-rpc --json-rpc-port 8545 --admin-via-http
