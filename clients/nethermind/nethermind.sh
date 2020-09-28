#!/bin/bash

# Startup script to initialize and boot a peer instance.
#
# This script assumes the following files:
#  - `nethermind` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID             network ID number to use for the eth protocol
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD       block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK       block number of the DAO hard-fork transitionnsition
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork
#  - HIVE_FORK_TANGERINE       block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS        block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM       block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE  block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG      block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_MINER                address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW             If set, skip PoW verification during block import

# Immediately abort the script on any error encountered
set -e

# Configure the chain.
mkdir -p /chainspec
jq -f /mapper.jq /genesis.json > /chainspec/test.json

# Set bootnode.
if [ -n "$HIVE_BOOTNODE" ]; then
    mkdir -p /nethermind/Data
    echo "[\"$HIVE_BOOTNODE\"]" > /nethermind/Data/static-nodes.json
fi

# Load any keys explicitly added to the node
#if [ -d ]; then
#	export NETHERMIND_HIVECONFIG_KEYSDIR=keys
#fi

echo "Running Nethermind..."
# The output is tee:d, via /log.txt, because the enode script uses that logfile to parse out the enode id
dotnet /nethermind/Nethermind.Runner.dll --config /configs/test.cfg 2>&1 | tee /log.txt
