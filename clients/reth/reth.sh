#!/bin/bash

# Startup script to initialize and boot a reth instance.
#
# This script assumes the following files:
#  - `reth` binary is located in the filesystem root
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
#  - HIVE_SHANGHAI_TIMESTAMP   timestamp for Shanghai transition
#  - HIVE_CANCUN_TIMESTAMP     timestamp for Cancun transition
#  - HIVE_LOGLEVEL             client log level
#
# These flags are NOT supported by reth
#
#  - HIVE_GRAPHQL_ENABLED      turns on GraphQL server
#  - HIVE_CLIQUE_PRIVATEKEY    private key for clique mining
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
#  - HIVE_MINER                address to credit with mining rewards
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks

# Immediately abort the script on any error encountered
set -ex

# no ansi colors
export RUST_LOG_STYLE=never

reth=/usr/local/bin/reth

case "$HIVE_LOGLEVEL" in
    0|1) FLAGS="$FLAGS -v" ;;
    2)   FLAGS="$FLAGS -vv" ;;
    3)   FLAGS="$FLAGS -vvv" ;;
    4)   FLAGS="$FLAGS -vvvv" ;;
    5)   FLAGS="$FLAGS -vvvvv" ;;
esac

# Create the data directory.
DATADIR="/reth-hive-datadir"
mkdir $DATADIR
FLAGS="$FLAGS --datadir $DATADIR"

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
echo "Initializing database with genesis state..."
$reth init $FLAGS --chain /genesis.json

# make sure we use the same genesis each time
FLAGS="$FLAGS --chain /genesis.json"

# Don't immediately abort, some imports are meant to fail
set +ex

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    RUST_LOG=info $reth import $FLAGS /chain.rlp
else
    echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
mapfile -t BLOCKS < <(ls /blocks/*.rlp 2>/dev/null | sort -n)

if [[ ! -d "/blocks" || ${#BLOCKS[@]} -eq 0 ]]; then
    echo "Warning: No blocks found."
elif [[ ${#BLOCKS[@]} -eq 1 ]]; then
    # Import the only existing block
    $reth import $FLAGS "${BLOCKS[0]}"
else
    # First import as many blocks as possible, and only then import the last one.
    # This is important because usually tests expecting a failure will assert that the last valid inserted block is at last - 1. If we attempted to import all of them the pipeline would unwind the whole range.
    cat "${BLOCKS[@]:0:${#BLOCKS[@]}-1}" > "combined.rlp"

    # Import all but the last block first
    $reth import $FLAGS "combined.rlp"

    # Import the last block separately
    $reth import $FLAGS "${BLOCKS[-1]}"
fi

# Only set boot nodes in online steps
# It doesn't make sense to dial out, use only a pre-set bootnode.
if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi

# Configure any mining operation
# TODO
#if [ "$HIVE_MINER" != "" ]; then
#    FLAGS="$FLAGS --mine --miner.etherbase $HIVE_MINER"
#fi
#if [ "$HIVE_MINER_EXTRA" != "" ]; then
#    FLAGS="$FLAGS --miner.extradata $HIVE_MINER_EXTRA"
#fi

# Import clique signing key.
# TODO
#if [ "$HIVE_CLIQUE_PRIVATEKEY" != "" ]; then
#    # Create password file.
#    echo "Importing clique key..."
#    echo "$HIVE_CLIQUE_PRIVATEKEY" > ./private_key.txt
#
#    # Ensure password file is used when running geth in mining mode.
#    if [ "$HIVE_MINER" != "" ]; then
#        FLAGS="$FLAGS --miner.sigfile private_key.txt"
#    fi
#fi

# If clique is expected enable auto-mine
if [ -n "${HIVE_CLIQUE_PRIVATEKEY}" ] || [ -n "${HIVE_CLIQUE_PERIOD}" ]; then
  FLAGS="$FLAGS --auto-mine"
  if [ -n "${HIVE_CLIQUE_PERIOD}" ]; then
    FLAGS="$FLAGS --dev.block-time ${HIVE_CLIQUE_PERIOD}s"
  fi
fi

# Configure RPC.
FLAGS="$FLAGS --http --http.addr=0.0.0.0 --http.api=admin,debug,eth,net,web3"
FLAGS="$FLAGS --ws --ws.addr=0.0.0.0 --ws.api=admin,debug,eth,net,web3"

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    JWT_SECRET="7365637265747365637265747365637265747365637265747365637265747365"
    echo -n $JWT_SECRET > /jwt.secret
    FLAGS="$FLAGS --authrpc.addr=0.0.0.0 --authrpc.jwtsecret=/jwt.secret"
fi

# Configure NAT and disable pruning
FLAGS="$FLAGS --nat none --block-interval 500000"

# Launch the main client.
echo "Running reth with flags: $FLAGS"
RUST_LOG=info $reth node $FLAGS
