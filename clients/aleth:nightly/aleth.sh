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
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW       If set, skip PoW verification during block import

# Immediately abort the script on any error encountered
set -e

# See http://www.ethdocs.org/en/latest/network/test-networks.html#custom-networks-eth
# for info about how it's configured

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --peerset $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --no-discovery"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
fi

# Configure and set the chain definition for the node
chainconfig=`cat /config.json`
genesis=`cat /genesis.json`
genesis="${genesis/coinbase/author}"

accounts=`echo $genesis | jq ".alloc"` && genesis=`echo $genesis | jq "del(.alloc)"`

# Remove unsupported genesis fields
genesis=`echo $genesis | jq "del(.bloom) | del(.hash) | del(.number) | del(.receiptTrie) | del(.stateRoot) | del(.transactionsTrie) | del(.uncleHash) | del(.gasUsed)"`

chainconfig=`echo $chainconfig | jq ". + {\"genesis\": $genesis}"`

# See https://github.com/ethcore/parity/wiki/Consensus-Engines for info about options


if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	HIVE_FORK_HOMESTEAD=`echo "obase=16; $HIVE_FORK_HOMESTEAD" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"homesteadForkBlock\"]; \"0x$HIVE_FORK_HOMESTEAD\")"`
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

if [ "$HIVE_FORK_BYZANTIUM" != "" ]; then
	HIVE_FORK_BYZANTIUM=`echo "obase=16; $HIVE_FORK_BYZANTIUM" | bc`

	if [ "$((16#$HIVE_FORK_BYZANTIUM))" -eq "0" ]; then
		# Also new precompiles
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000005\"]; { \"precompiled\": { \"name\": \"modexp\" } })"`
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000006\"]; { \"precompiled\": { \"name\": \"alt_bn128_G1_add\", \"linear\": { \"base\": 500, \"word\": 0 } } })"`
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000007\"]; { \"precompiled\": { \"name\": \"alt_bn128_G1_mul\", \"linear\": { \"base\": 40000, \"word\": 0 } } })"`
		chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000008\"]; { \"precompiled\": { \"name\": \"alt_bn128_pairing_product\" } })"`
	fi

	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"byzantiumForkBlock\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`
fi

if [ "$HIVE_FORK_CONSTANTINOPLE" != "" ]; then
	HIVE_FORK_CONSTANTINOPLE=`echo "obase=16; $HIVE_FORK_CONSTANTINOPLE" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([ \"params\", \"constantinopleForkBlock\"]; \"0x$HIVE_FORK_CONSTANTINOPLE\")"`
fi

if [ "$accounts" != "" ]; then
	# In some cases, the 'alloc' portion can be extremely large.
	# Because of this, it can't be handled via cmd line parameters,
	# this fails:
	# chainconfig=`echo $chainconfig | jq ". * {\"accounts\": $accounts}"`
	# The following solution instead uses two temporary files
	echo $chainconfig > tmp1
	echo $accounts| jq "{ \"accounts\": .}" > tmp2
	chainconfig=`jq -s '.[0] * .[1]' tmp1 tmp2`
fi

# For some reason, aleth doesn't like it when precompiles have code/nonce/storage, so we need to remove those fields. 
for i in $(seq 1 8); do
	chainconfig=`echo $chainconfig | jq "delpaths([[\"accounts\", \"000000000000000000000000000000000000000$i\", \"code\"]])"`
	chainconfig=`echo $chainconfig | jq "delpaths([[\"accounts\", \"000000000000000000000000000000000000000$i\", \"nonce\"]])"`
	chainconfig=`echo $chainconfig | jq "delpaths([[\"accounts\", \"000000000000000000000000000000000000000$i\", \"storage\"]])"`
done

if [ "$HIVE_SKIP_POW" != "" ]; then
	chainconfig=`echo $chainconfig | jq "setpath([\"sealEngine\"];\"NoProof\")"`
fi

echo $chainconfig > /chain2.json

# Initialize the local testchain with the genesis state
FLAGS="$FLAGS --config /chain2.json"
ETHEXEC=/usr/bin/aleth
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
		echo "Command: eth $FLAGS --import /blocks/$block"
		$ETHEXEC $FLAGS --import /blocks/$block
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

# Run the client with the requested flags
echo "Running Aleth... "

RUNCMD="python3 /usr/bin/aleth.py --rpc http://0.0.0.0:8545 --aleth-exec $ETHEXEC $FLAGS"
echo "cmd: $RUNCMD"
$RUNCMD
