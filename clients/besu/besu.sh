#!/bin/bash

# Startup script to initialize and boot a peer instance.
#
# This script assumes the following files:
#  - `genesis.json` file is located at /etc/besu/genesis.json (mandatory)
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID             network ID number to use for the eth protocol
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD       block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK       block number of the DAO hard-fork transitionnsition
#  - HIVE_FORK_TANGERINE       block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS        block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM       block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE  block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG      block number for ConstantinopleFix/Petersburg transition
#  - HIVE_FORK_ISTANBUL        block number for Istanbul transition
#  - HIVE_FORK_MUIR_GLACIER    block number for MuirGlacier transition
#  - HIVE_FORK_BERLIN          block number for Berlin transition
#  - HIVE_MINER                address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW             If set, skip PoW verification
#  - HIVE_LOGLEVEL             Client log level
#  - HIVE_GRAPHQL_ENABLED      If set, GraphQL is enabled on port 8545 and RPC is disabled
#
# These flags are not supported by the Besu hive client
#
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork

set -e

besu=/opt/besu/bin/besu

# See https://github.com/hyperledger/besu/issues/1464
export BESU_OPTS="-Dsecp256k1.randomize=false"

# Configure logging.
LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac
FLAGS="--logging=$LOG"

# Configure the chain.
jq -f /mapper.jq /genesis.json > /besugenesis.json
echo -n "Genesis: "; cat /besugenesis.json
FLAGS="$FLAGS --genesis-file=/besugenesis.json"

# Enable experimental 'berlin' hard-fork features if configured.
if [ -n "$HIVE_FORK_BERLIN" ]; then
    FLAGS="$FLAGS --Xberlin-enabled=true"
fi

# The client should start after loading the blocks, this option configures it.
IMPORTFLAGS="--run"

# Disable PoW check if requested.
if [ -n "$HIVE_SKIP_POW" ]; then
    IMPORTFLAGS="$IMPORTFLAGS --skip-pow-validation-enabled"
fi

# Load chain.rlp if present.
if [ -f /chain.rlp ]; then
    HAS_IMPORT=1
    IMPORTFLAGS="$IMPORTFLAGS --from=/chain.rlp"
fi

# Load the remaining individual blocks.
if [ -d /blocks ]; then
    HAS_IMPORT=1
    blocks=`echo /blocks/* | sort -n`
    IMPORTFLAGS="$IMPORTFLAGS $blocks"
fi

# Configure mining.
if [ "$HIVE_MINER" != "" ]; then
    FLAGS="$FLAGS --miner-enabled --miner-coinbase=$HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
    FLAGS="$FLAGS --miner-extra-data=$HIVE_MINER_EXTRA"
fi
FLAGS="$FLAGS --min-gas-price=16 --tx-pool-price-bump=0"

# Configure peer-to-peer networking.
if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi
if [ "$HIVE_NETWORK_ID" != "" ]; then
    FLAGS="$FLAGS --network-id=$HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --network-id=1337"
fi

# Sync mode.
case "$HIVE_NODETYPE" in
    "" | "full")
        FLAGS="$FLAGS --sync-mode=FAST --fast-sync-min-peers=1 --Xsynchronizer-fast-sync-pivot-distance=0" ;;
    "light")
        echo "Ignoring HIVE_NODETYPE == light: besu does not support light client" ;;
esac

# Configure RPC.
RPCFLAGS="--host-whitelist=*"
if [ "$HIVE_GRAPHQL_ENABLED" == "" ]; then
    RPCFLAGS="$RPCFLAGS --rpc-http-enabled --rpc-http-api=ETH,NET,WEB3,ADMIN --rpc-http-host=0.0.0.0"
else
    RPCFLAGS="$RPCFLAGS --rpc-http-port=8550" # work around duplicate port error
    RPCFLAGS="$RPCFLAGS --graphql-http-enabled --graphql-http-host=0.0.0.0 --graphql-http-port=8545"
fi

# Start Besu.
if [ -z "$HAS_IMPORT" ]; then
    cmd="$besu $FLAGS $RPCFLAGS"
else
    cmd="$besu $FLAGS $RPCFLAGS blocks import $IMPORTFLAGS"
fi
echo "starting main client: $cmd"
$cmd
