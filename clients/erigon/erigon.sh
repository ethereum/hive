#!/bin/bash

# Startup script to initialize and boot an erigon instance.
#
# This script assumes the following files:
#  - `erigon` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#
# This script can be configured using the following environment variables:
#
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_FORK_HOMESTEAD       block number of the homestead transition
#  - HIVE_FORK_DAO_BLOCK       block number of the DAO hard-fork transition
#  - HIVE_FORK_TANGERINE       block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS        block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM       block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE  block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG      block number for ConstantinopleFix/Petersburg transition
#  - HIVE_FORK_ISTANBUL        block number for Istanbul transition
#  - HIVE_FORK_MUIR_GLACIER    block number for MuirGlacier transition
#  - HIVE_SKIP_POW             If set, skip PoW verification during block import
#  - HIVE_LOGLEVEL             client log level
#  - HIVE_MINER                address to credit with mining rewards
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#
# These flags are NOT supported by Erigon
#
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_GRAPHQL_ENABLED      turns on GraphQL server
#  - HIVE_CLIQUE_PRIVATEKEY    private key for clique mining
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)

# Immediately abort the script on any error encountered
set -e

erigon=/usr/local/bin/erigon

if [ "$HIVE_LOGLEVEL" != "" ]; then
    FLAGS="$FLAGS --log.console.verbosity=$HIVE_LOGLEVEL"
fi

if [ "$HIVE_BOOTNODE" != "" ]; then
    # Somehow the bootnodes flag is not working for erigon, only staticpeers is working for sync tests
    FLAGS="$FLAGS --staticpeers $HIVE_BOOTNODE --nodiscover"
fi

if [ "$HIVE_SKIP_POW" != "" ]; then
    FLAGS="$FLAGS --fakepow"
fi

# Create the data directory.
mkdir /erigon-hive-datadir
FLAGS="$FLAGS --datadir /erigon-hive-datadir"
FLAGS="$FLAGS --db.size.limit 2GB"

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
    FLAGS="$FLAGS --networkid $HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --networkid 1337"
fi

# Configure the chain.
mv /genesis.json /genesis-input.json
jq -f /mapper.jq /genesis-input.json > /genesis.json

# Dump genesis
echo "Supplied genesis state:"
cat /genesis.json

echo "Command flags till now:"
echo $FLAGS

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
$erigon $FLAGS init /genesis.json

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    echo "Loading initial blockchain..."
    $erigon $FLAGS import /chain.rlp
else
    echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
    echo "Loading remaining individual blocks..."
    for file in $(ls /blocks | sort -n); do
        echo "Importing " $file
        $erigon $FLAGS import /blocks/$file
    done
else
    echo "Warning: blocks folder not found."
fi

set -e

# Configure any mining operation
# TODO: Erigon doesn't have inbuilt cpu miner. Need to add https://github.com/panglove/ethcpuminer/tree/master/ethash for cpu mining with erigon
if [ "$HIVE_MINER" != "" ]; then
    FLAGS="$FLAGS --mine --miner.etherbase $HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
    FLAGS="$FLAGS --miner.extradata $HIVE_MINER_EXTRA"
fi

# Import clique signing key.
if [ "$HIVE_CLIQUE_PRIVATEKEY" != "" ]; then
    # Create password file.
    echo "Importing clique key..."
    echo "$HIVE_CLIQUE_PRIVATEKEY" > ./private_key.txt

    # Ensure password file is used when running geth in mining mode.
    if [ "$HIVE_MINER" != "" ]; then
        FLAGS="$FLAGS --miner.sigfile private_key.txt"
    fi
fi

# Configure RPC.
FLAGS="$FLAGS --http --http.addr=0.0.0.0 --http.api=admin,debug,eth,net,txpool,web3"
FLAGS="$FLAGS --ws"

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    JWT_SECRET="0x7365637265747365637265747365637265747365637265747365637265747365"
    echo -n $JWT_SECRET > /jwt.secret
    FLAGS="$FLAGS --authrpc.addr=0.0.0.0 --authrpc.jwtsecret=/jwt.secret"
fi

# Launch the main client.
FLAGS="$FLAGS --nat=none"
echo "Running erigon with flags $FLAGS"
$erigon $FLAGS
