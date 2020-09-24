#!/bin/bash

# Startup script to initialize and boot a peer instance.
#
# This script assumes the following files:
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)

# This script can be configured using the following environment variables:
#
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID             network ID number to use for the eth protocol
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
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
#
# These flags are NOT supported by Trinity
#
#  - HIVE_FORK_BERLIN          block number for Berlin transition
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork
#  - HIVE_MINER                address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#  - HIVE_GRAPHQL_ENABLED      turns on GraphQL server

# Immediately abort the script on any error encountered
set -e

# Configure log level.
level="DEBUG"
case "$HIVE_LOGLEVEL" in
    0) level=CRITICAL ;;
    1) level=ERROR    ;;
    2) level=WARN     ;;
    3) level=INFO     ;;
    4) level=DEBUG    ;;
    5) level=DEBUG2   ;;
esac
FLAGS="$FLAGS --log-level $level"

# Create the data directory.
mkdir /dd /dd/logs /dd/logs-eth1
FLAGS="$FLAGS --data-dir /dd"

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
    FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --network-id 1337"
fi

# Configure and set the chain definition.
jq -f /mapper.jq /genesis.json > /newconfig.json
FLAGS="$FLAGS --genesis /newconfig.json"

# Disable TxPool. Trinity won't start with a custom genesis unless
# this option is also given.
FLAGS="$FLAGS --disable-tx-pool"

# Don't immediately abort, some imports are meant to fail.
set +e

# Import blockchain from /chain.rlp and /blocks.
if [ -f /chain.rlp ]; then
    echo "Loading initial blockchain..."
    trinity $FLAGS import /chain.rlp
fi
if [ -d /blocks ]; then
    echo "Loading remaining individual blocks..."
    for file in $(ls /blocks | sort -n); do
        echo "Importing " $file
        trinity $FLAGS import /blocks/$file
    done
fi

set -e

# Enable the HTTP server.
FLAGS="$FLAGS --http-listen-address 0.0.0.0 --http-port 8545 --enable-http-apis eth,net,admin"

# Configure peer-to-peer networking.
if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --preferred-node $HIVE_BOOTNODE"
fi
FLAGS="$FLAGS --disable-upnp"

# Configure sync mode.
mode="full"
case "$HIVE_NODETYPE" in
    full|archive) mode=full  ;;
    light)        mode=light ;;
esac
FLAGS="$FLAGS --sync-mode $mode"

# Run the client.
echo "Running Trinity... "
echo "flags: $FLAGS"
trinity $FLAGS
