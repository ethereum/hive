#!/bin/bash

# Startup script to initialize and boot a ethereum-js instance.
#
# This script assumes the following files:
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#
#  - HIVE_BOOTNODE                enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID              network ID number to use for the eth protocol
#  - HIVE_NODETYPE                sync and pruning selector (archive, full, light)
#
# Forks:
#
#  - HIVE_FORK_HOMESTEAD          block number of the homestead hard-fork transition
#  - HIVE_FORK_DAO_BLOCK          block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_VOTE           whether the node support (or opposes) the DAO fork
#  - HIVE_FORK_TANGERINE          block number of Tangerine Whistle transition
#  - HIVE_FORK_SPURIOUS           block number of Spurious Dragon transition
#  - HIVE_FORK_BYZANTIUM          block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE     block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG         block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_FORK_ISTANBUL           block number for Istanbul transition
#  - HIVE_FORK_MUIRGLACIER        block number for Muir Glacier transition
#  - HIVE_FORK_BERLIN             block number for Berlin transition
#  - HIVE_FORK_LONDON             block number for London
#
# Clique PoA:
#
#  - HIVE_CLIQUE_PERIOD           enables clique support. value is block time in seconds.
#  - HIVE_CLIQUE_PRIVATEKEY       private key for clique mining
#
# Other:
#
#  - HIVE_MINER                   enable mining. value is coinbase address.
#  - HIVE_MINER_EXTRA             extra-data field to set for newly minted blocks
#  - HIVE_LOGLEVEL                client loglevel (0-5)
#  - HIVE_GRAPHQL_ENABLED         enables graphql on port 8545
#  - HIVE_LES_SERVER              set to '1' to enable LES server


# Immediately abort the script on any error encountered
set -e

CLIENT_DIRECTORY=/ethereumjs-monorepo/packages/client

ethereumjs="node $CLIENT_DIRECTORY/dist/esm/bin/cli.js"
FLAGS="--gethGenesis ./genesis.json --rpc --rpcEngine --saveReceipts --rpcAddr 0.0.0.0 --rpcEngineAddr 0.0.0.0 --rpcEnginePort 8551 --ws false --logLevel debug --rpcDebug all --rpcDebugVerbose all --isSingleNode"

# Configure the chain.
mv /genesis.json /genesis-input.json
jq -f /mapper.jq /genesis-input.json > ./genesis.json

# Dump genesis. 
if [ "$HIVE_LOGLEVEL" -lt 4 ]; then
    echo "Supplied genesis state (trimmed, use --sim.loglevel 4 or 5 for full output):"
    jq 'del(.alloc[] | select(.balance == "0x123450000000000000000"))' ./genesis.json
else
    echo "Supplied genesis state:"
    cat ./genesis.json
fi

# Import clique signing key.
if [ "$HIVE_CLIQUE_PRIVATEKEY" != "" ]; then
    # Create password file.
    echo "Importing clique key..."
    echo -n "$HIVE_CLIQUE_PRIVATEKEY" > ./private_key.txt
    # Ensure password file is used when running ethereumjs in mining mode.
    if [ "$HIVE_MINER" != "" ]; then
        FLAGS="$FLAGS --mine --unlock ./private_key.txt --minerCoinbase 0x$HIVE_MINER"
    fi
fi

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    FLAGS="$FLAGS --jwtSecret ./jwtsecret"
fi

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
  FLAGS="$FLAGS --loadBlocksFromRlp=/chain.rlp"
  else
  echo "Warning: chain.rlp not found."
fi

if [[ -d blocks ]]; then
  for file in blocks/*; do
    FLAGS="$FLAGS --loadBlocksFromRlp=${file}" 
  done
  else
  echo "Warning: blocks directory not found."
fi

if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi
echo "Running ethereumjs with flags $FLAGS"


$ethereumjs $FLAGS
