#!/bin/bash

# Startup script to initialize and boot a parity instance.
#
# This script assumes the following files:
#  - `openethereum` binary is located in the filesystem root
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
#  - HIVE_LOGLEVEL       Sets the log level

# Immediately abort the script on any error encountered
set -e

# Configure and set the chain definition for the node
jq -f /mapper.jq /genesis.json > /chain.json
echo -n "Chain spec: "
cat /chain.json
echo "----------"

OE="/home/openethereum/openethereum"
FLAGS="--chain /chain.json"

LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4|5) LOG=debug ;;
    # OpenEthereum trace logging is INSANE, that's why it's disabled here.
    # 5) LOG=trace ;;
esac
FLAGS="$FLAGS -l $LOG"

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_SKIP_POW" != "" ]; then
    FLAGS="$FLAGS --no-seal-check"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
    FLAGS="$FLAGS --network-id $HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --network-id 1337"
fi

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    $OE $FLAGS import /chain.rlp
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
    for block in `ls /blocks | sort -n`; do
        echo "parity $FLAGS import /blocks/$block"
        $OE $FLAGS import /blocks/$block
    done
fi

# Immediately abort the script on any error encountered
set -e

if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
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

# Run OpenEthereum with the requested flags.
FLAGS="$FLAGS --no-warp --usd-per-eth 1 --nat none --jsonrpc-interface all --jsonrpc-hosts all  --jsonrpc-apis all"
echo "running $OE $FLAGS"
$OE $FLAGS
