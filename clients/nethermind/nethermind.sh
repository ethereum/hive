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
#
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID             network ID number to use for the eth protocol
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
#  - HIVE_SKIP_POW             If set, skip PoW verification during block import
#
# Forks:
#
#  - HIVE_FORK_HOMESTEAD       block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK       block number of the DAO hard-fork transitionnsition
#  - HIVE_FORK_TANGERINE       block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS        block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM       block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE  block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG      block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_FORK_BERLIN          block number for Berlin transition
#  - HIVE_FORK_LONDON          block number for London
#
# Clique PoA:
#
#  - HIVE_CLIQUE_PERIOD        enables clique support. value is block time in seconds.
#  - HIVE_CLIQUE_PRIVATEKEY    private key for clique mining
#
# Other:
#
#  - HIVE_MINER                enables mining. value is coinbase address.
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#  - HIVE_LOGLEVEL             Client log level
#
# These variables are not supported by Nethermind:
#
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork
#  - HIVE_GRAPHQL_ENABLED      if set, GraphQL is enabled on port 8545
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed

# Immediately abort the script on any error encountered
set -e

# Generate JWT file if necessary
if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    JWT_SECRET="0x7365637265747365637265747365637265747365637265747365637265747365"
    echo -n $JWT_SECRET > /jwt.secret
fi

# Generate the genesis and chainspec file.
mkdir -p /chainspec
jq -f /mapper.jq /genesis.json > /chainspec/test.json
jq . /chainspec/test.json

# Generate the config file.
mkdir /configs
echo "Supplied genesis state:" 
jq -n -f /mkconfig.jq > /configs/test.cfg

echo "test.cfg"
cat /configs/test.cfg

# Set bootnode.
if [ -n "$HIVE_BOOTNODE" ]; then
    mkdir -p /nethermind/Data
    echo "[\"$HIVE_BOOTNODE\"]" > /nethermind/Data/static-nodes.json
fi

# Configure logging.
LOG_FLAG=""
if [ "$HIVE_LOGLEVEL" != "" ]; then
    case "$HIVE_LOGLEVEL" in
        0)   LOG=OFF ;;
        1)   LOG=ERROR ;;
        2)   LOG=WARN  ;;
        3)   LOG=INFO  ;;
        4)   LOG=DEBUG ;;
        5)   LOG=TRACE ;;
    esac
    LOG_FLAG="--log $LOG"
fi
echo "Running Nethermind..."
# The output is tee:d, via /log.txt, because the enode script uses that logfile to parse out the enode id
dotnet /nethermind/Nethermind.Runner.dll --config /configs/test.cfg $LOG_FLAG 2>&1 | tee /log.txt
