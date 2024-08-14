#!/bin/bash

# Immediately abort the script on any error encountered
set -ex

# no ansi colors
export RUST_LOG_STYLE=never

ethereum_rust=./ethereum_rust

# Create the data directory.
# DATADIR="/ethereum-rust-hive-datadir"
# mkdir $DATADIR
# FLAGS="$FLAGS --datadir $DATADIR"

# TODO If a specific network ID is requested, use that
#if [ "$HIVE_NETWORK_ID" != "" ]; then
#    FLAGS="$FLAGS --networkid $HIVE_NETWORK_ID"
#else
#    FLAGS="$FLAGS --networkid 1337"
#fi

# Configure the chain.
mv /genesis.json /genesis-input.json
jq -f /mapper.jq /genesis-input.json > /genesis.json

# Dump genesis.
if [ "$HIVE_LOGLEVEL" -lt 4 ]; then
    echo "Supplied genesis state (trimmed, use --sim.loglevel 4 or 5 for full output):"
    jq 'del(.alloc[] | select(.balance == "0x123450000000000000000"))' /genesis.json
else
    echo "Supplied genesis state:"
    cat /genesis.json
fi

echo "Command flags till now:"
echo $FLAGS

# Initialize the local testchain with the genesis state
# echo "Initializing database with genesis state..."
# $ethereum_rust init $FLAGS --chain /genesis.json
FLAGS="$FLAGS --network /genesis.json"

# Don't immediately abort, some imports are meant to fail
set +ex

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    echo "Importing chain.rlp..."
    FLAGS="$FLAGS --import /chain.rlp"
    # $ethereum_rust import $FLAGS /chain.rlp
else
    echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
if [ -d /blocks ]; then
    echo "Loading remaining individual blocks..."
    for file in $(ls /blocks | sort -n); do
        echo "Importing " $file
        # $ethereum_rust import $FLAGS /blocks/$file
    done
else
    echo "Warning: blocks folder not found."
fi

# Only set boot nodes in online steps
# It doesn't make sense to dial out, use only a pre-set bootnode.
# if [ "$HIVE_BOOTNODE" != "" ]; then
#     FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
# fi

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
   echo "Warning: miner not supported."
   exit 1
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
    echo "Warning: miner extra data not supported."
    exit 1
fi

# Import clique signing key.
if [ "$HIVE_CLIQUE_PRIVATEKEY" != "" ]; then
    echo "Warning: clique private key not supported."
    exit 1
fi

# Configure RPC.
# FLAGS="$FLAGS --http --http.addr=0.0.0.0 --http.api=admin,debug,eth,net,web3"
# FLAGS="$FLAGS --ws --ws.addr=0.0.0.0 --ws.api=admin,debug,eth,net,web3"
FLAGS="$FLAGS --http.addr=0.0.0.0  --authrpc.addr=0.0.0.0"

# if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
#     JWT_SECRET="7365637265747365637265747365637265747365637265747365637265747365"
#     echo -n $JWT_SECRET > /jwt.secret
#     FLAGS="$FLAGS --authrpc.addr=0.0.0.0 --authrpc.jwtsecret=/jwt.secret"
# fi

# Configure NAT
# FLAGS="$FLAGS --nat none"

# Launch the main client.
echo "Running ethereum_rust with flags: $FLAGS"
RUST_LOG=info $ethereum_rust $FLAGS
