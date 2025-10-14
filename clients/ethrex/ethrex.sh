#!/bin/bash

# Immediately abort the script on any error encountered
set -ex

# no ansi colors
export RUST_LOG_STYLE=never

ethrex=./ethrex

# Configure the chain.
mv /genesis.json /genesis-input.json
jq -f /mapper.jq /genesis-input.json > /genesis.json

# Configure log level
LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac
FLAGS="$FLAGS --log.level=$LOG"

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
# $ethrex init $FLAGS --chain /genesis.json
FLAGS="$FLAGS --network /genesis.json"

# Don't immediately abort, some imports are meant to fail
set +ex

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    echo "Importing chain.rlp..."
    $ethrex $FLAGS $HIVE_ETHREX_FLAGS import /chain.rlp
else
    echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
if [ -d /blocks ]; then
    echo "Loading remaining individual blocks..."
    $ethrex $FLAGS $HIVE_ETHREX_FLAGS import /blocks
else
    echo "Warning: blocks folder not found."
fi

# Only set boot nodes in online steps
# It doesn't make sense to dial out, use only a pre-set bootnode.
if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi

# Configure sync mode.
case "$HIVE_NODETYPE" in
    "" | full | archive)
        syncmode="full" ;;
    snap)
        syncmode="snap" ;;
    *)
        echo "Unsupported HIVE_NODETYPE = $HIVE_NODETYPE"
        exit 1 ;;
esac
FLAGS="$FLAGS --syncmode=$syncmode"

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
FLAGS="$FLAGS --http.addr=0.0.0.0  --authrpc.addr=0.0.0.0"

# We don't support pre merge
if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    JWT_SECRET="7365637265747365637265747365637265747365637265747365637265747365"
    echo -n $JWT_SECRET > /jwt.secret
    FLAGS="$FLAGS  --authrpc.jwtsecret=/jwt.secret"
else
    # We dont exit because some tests require this
    echo "Warning: HIVE_TERMINAL_TOTAL_DIFFICULTY not supported."
fi

# Set amount of milliseconds between each transaction broadcast
FLAGS="$FLAGS --p2p.tx-broadcasting-interval=5"

FLAGS="$FLAGS  $HIVE_ETHREX_FLAGS"

# Launch the main client.
echo "Running ethrex with flags: $FLAGS"
$ethrex $FLAGS
