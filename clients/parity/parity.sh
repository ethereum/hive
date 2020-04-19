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
#  - HIVE_NETWORK_ID     network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID     network ID number to use for the eth protocol
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
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW       If set, skip PoW verification during block import

# Immediately abort the script on any error encountered
set -e

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --no-discovery"
fi

if [ "$HIVE_SKIP_POW" != "" ]; then
	FLAGS="$FLAGS --no-seal-check"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --network-id 1337"
fi

# Configure and set the chain definition for the node
chainconfig=`cat /chain.json`

genesis=`cat /genesis.json`
genesis="${genesis/coinbase/author}"

accounts=`echo $genesis | jq ".alloc"` && genesis=`echo $genesis | jq "del(.alloc)"`
nonce=`echo $genesis | jq ".nonce"` && genesis=`echo $genesis | jq "del(.nonce)"`
# Nonce needs to be padded
# First we snip off the quote-char and the '0x' prefix
nonce=`echo $nonce| cut -c 4-`
# Then the last quote char
nonce=`echo ${nonce%?}`
# pad it with zeroes
while [ ${#nonce} -lt 16 ]; do
  nonce=0$nonce
done
# Put back 0x
nonce=0x$nonce
mixhash=`echo $genesis | jq ".mixHash"` && genesis=`echo $genesis | jq "del(.mixHash)"`
genesis=`echo $genesis | jq ". + {\"seal\": {\"ethereum\": {\"nonce\": \"$nonce\", \"mixHash\": $mixhash}}}"| jq "del (.config, .number)"`

if [ "$accounts" != "" ]; then
	# In some cases, the 'alloc' portion can be extremely large.
	# Because of this, it can't be handled via cmd line parameters,
	# this fails:
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
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"homesteadTransition\"]; \"0x789b0\")"`
fi
if [ "$HIVE_CHAIN_ID" != "" ]; then
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"chainID\"]; \"0x$HIVE_CHAIN_ID\")"`
fi
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	HEX_HIVE_FORK_HOMESTEAD=`echo "obase=16; $HIVE_FORK_HOMESTEAD" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"homesteadTransition\"]; \"0x$HEX_HIVE_FORK_HOMESTEAD\")"`
fi

if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
	HIVE_FORK_DAO_BLOCK=`echo "obase=16; $HIVE_FORK_DAO_BLOCK" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"daoHardforkTransition\"]; \"0x$HIVE_FORK_DAO_BLOCK\")"`
fi

if [ "$HIVE_FORK_TANGERINE" != "" ]; then
	HIVE_FORK_TANGERINE=`echo "obase=16; $HIVE_FORK_TANGERINE" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip150Transition\"]; \"0x$HIVE_FORK_TANGERINE\" )"`
fi
# if [ "$HIVE_FORK_TANGERINE_HASH" != "" ]; then
# 	HIVE_FORK_TANGERINE_HASH=`echo "obase=16; $HIVE_FORK_TANGERINE_HASH" | bc`
# 	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip150Transition\"]; \"0x$HIVE_FORK_TANGERINE_HASH\" )"`
# fi


if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
	HIVE_FORK_SPURIOUS=`echo "obase=16; $HIVE_FORK_SPURIOUS" | bc`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip155Transition\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip160Transition\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip161abcTransition\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip161dTransition\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"maxCodeSizeTransition\"]; \"0x$HIVE_FORK_SPURIOUS\")"`
fi

# independent of the forks
chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000006\", \"builtin\"]; { \"name\": \"alt_bn128_add\",  \"pricing\": {  } })"`
chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000007\", \"builtin\"]; { \"name\": \"alt_bn128_mul\",  \"pricing\": {  } })"`
chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000008\", \"builtin\"]; { \"name\": \"alt_bn128_pairing\",  \"pricing\": { } })"`


if [ "$HIVE_FORK_BYZANTIUM" != "" ]; then
	# Based on 
	# https://github.com/paritytech/parity/blob/metropolis-update/ethcore/res/ethereum/byzantium_test.json

	HIVE_FORK_BYZANTIUM=`echo "obase=16; $HIVE_FORK_BYZANTIUM" | bc`
#	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip98Transition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`

	# Ethash params
	#  Set blockreward to 3M for byzantium
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"blockReward\",\"0x$HIVE_FORK_BYZANTIUM\"]; \"0x29A2241AF62C0000\")"`
	# difficulty calculation -- aka bomb delay
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"eip100bTransition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`

	# General params
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip140Transition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip211Transition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip214Transition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip658Transition\"]; \"0x$HIVE_FORK_BYZANTIUM\")"`


	# Precompiles
	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000005\", \"builtin\"]; { \"name\": \"modexp\", \"activate_at\": \"0x$HIVE_FORK_BYZANTIUM\", \"pricing\": { \"modexp\": { \"divisor\": 20 }  } })"`

	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000006\", \"builtin\", \"pricing\", \"0x$HIVE_FORK_BYZANTIUM\" ]; { \"price\": { \"alt_bn128_const_operations\": { \"price\": 500 }}   } )"`
	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000007\", \"builtin\", \"pricing\", \"0x$HIVE_FORK_BYZANTIUM\" ]; { \"price\": { \"alt_bn128_const_operations\": { \"price\": 40000 }}   } )"`
	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000008\", \"builtin\",\"pricing\", \"0x$HIVE_FORK_BYZANTIUM\" ]; { \"price\": { \"alt_bn128_pairing\":  { \"base\": 100000, \"pair\": 80000 }}   } )"`




fi
if [ "$HIVE_FORK_CONSTANTINOPLE" != "" ]; then
	# New shift instructions
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip145Transition\"]; \"0x$HIVE_FORK_CONSTANTINOPLE\")"`
	# EXTCODEHASH
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1052Transition\"]; \"0x$HIVE_FORK_CONSTANTINOPLE\")"`
	# EIP 1283, net gas metering version 2

	# Skinny create 2
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1014Transition\"]; \"0x$HIVE_FORK_CONSTANTINOPLE\")"`
	# Ethash params
	#  Set blockreward to 2M for constantinople
	chainconfig=`echo $chainconfig | jq "setpath([\"engine\", \"Ethash\", \"params\", \"blockReward\",\"0x$HIVE_FORK_CONSTANTINOPLE\"]; \"0x1BC16D674EC80000\")"`

fi
# if [ "$HIVE_FORK_PETERSBURG" != "" ]; then
# 	# EIP 1283 disabling
# 	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1283DisableTransition\"]; \"0x$HIVE_FORK_PETERSBURG\")"`
# fi
if [ "$HIVE_FORK_ISTANBUL" != "" ]; then
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1283DisableTransition\"]; \"0x0\")"`
	 chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1283ReenableTransition\"]; \"0x$HIVE_FORK_ISTANBUL\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1283Transition\"]; \"0x0\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1344Transition\"]; \"0x$HIVE_FORK_ISTANBUL\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1706Transition\"]; \"0x$HIVE_FORK_ISTANBUL\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip1884Transition\"]; \"0x$HIVE_FORK_ISTANBUL\")"`
	chainconfig=`echo $chainconfig | jq "setpath([\"params\", \"eip2028Transition\"]; \"0x$HIVE_FORK_ISTANBUL\")"`

	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000006\", \"builtin\", \"pricing\", \"0x$HIVE_FORK_ISTANBUL\" ]; { \"price\": { \"alt_bn128_const_operations\": { \"price\": 150 }}   } )"`
	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000007\", \"builtin\", \"pricing\", \"0x$HIVE_FORK_ISTANBUL\" ]; { \"price\": { \"alt_bn128_const_operations\": { \"price\": 6000 }}   } )"`
	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000008\", \"builtin\",\"pricing\", \"0x$HIVE_FORK_ISTANBUL\" ]; { \"price\": { \"alt_bn128_pairing\":  { \"base\": 45000, \"pair\": 34000 }}   } )"`


	chainconfig=`echo $chainconfig | jq "setpath([\"accounts\", \"0000000000000000000000000000000000000009\", \"builtin\"]; { \"name\": \"blake2_f\", \"activate_at\": \"0x$HIVE_FORK_ISTANBUL\", \"pricing\": { \"blake2_f\": { \"gas_per_round\": 1} } })"`

fi

echo $chainconfig > /chain.json
echo "Chain config: "
cat /chain.json
echo "----------"

FLAGS="$FLAGS --chain /chain.json"

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	parity $FLAGS import /chain.rlp
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	for block in `ls /blocks | sort -n`; do
		echo "parity $FLAGS import /blocks/$block"
		parity $FLAGS import /blocks/$block
	done
fi

# Immediately abort the script on any error encountered
set -e

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
parity $FLAGS  --no-warp --usd-per-eth 1 --nat none --jsonrpc-interface all --jsonrpc-hosts all
