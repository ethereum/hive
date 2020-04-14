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

# It doesn't make sense to dial out, use only a pre-set bootnode
# TODO - 
if [ "$HIVE_BOOTNODE" != "" ]; then
	export NETHERMIND_HIVECONFIG_BOOTNODE=$HIVE_BOOTNODE
fi

# Override any chain configs in the go-ethereum specific way
chainconfig="{}"

if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	echo "Setting homestead block:$HIVE_FORK_HOMESTEAD"
	chainconfig=`echo $chainconfig | jq ". + {\"homesteadBlock\": $HIVE_FORK_HOMESTEAD}"`
fi
if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
	echo "Setting DAO block:$HIVE_FORK_DAO_BLOCK"
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkBlock\": $HIVE_FORK_DAO_BLOCK}"`
fi
if [ "$HIVE_FORK_DAO_VOTE" == "0" ]; then
	echo "Setting dao vote block:$HIVE_FORK_DAO_VOTE"
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkSupport\": false}"`
fi
if [ "$HIVE_FORK_DAO_VOTE" == "1" ]; then
	echo "Setting dao vot block:$HIVE_FORK_DAO_VOTE"
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkSupport\": true}"`
fi
if [ "$HIVE_FORK_TANGERINE" != "" ]; then
	echo "Setting tangerine block:$HIVE_FORK_TANGERINE"
	chainconfig=`echo $chainconfig | jq ". + {\"eip150Block\": $HIVE_FORK_TANGERINE}"`
fi
if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
	echo "Setting spurious block:$HIVE_FORK_SPURIOUS"
	chainconfig=`echo $chainconfig | jq ". + {\"eip158Block\": $HIVE_FORK_SPURIOUS}"`
	chainconfig=`echo $chainconfig | jq ". + {\"eip155Block\": $HIVE_FORK_SPURIOUS}"`
fi
if [ "$HIVE_FORK_BYZANTIUM" != "" ]; then
	echo "Setting byzantium block:$HIVE_FORK_BYZANTIUM"
	chainconfig=`echo $chainconfig | jq ". + {\"byzantiumBlock\": $HIVE_FORK_BYZANTIUM}"`
fi
if [ "$HIVE_FORK_CONSTANTINOPLE" != "" ]; then
	echo "Setting constantinople block:$HIVE_FORK_CONSTANTINOPLE"
	chainconfig=`echo $chainconfig | jq ". + {\"constantinopleBlock\": $HIVE_FORK_CONSTANTINOPLE}"`
fi
if [ "$HIVE_FORK_PETERSBURG" != "" ]; then
    echo "Setting petersburg block:$HIVE_FORK_PETERSBURG"
	chainconfig=`echo $chainconfig | jq ". + {\"petersburgBlock\": $HIVE_FORK_PETERSBURG}"`
fi

genesis=`cat /genesis.json` 
echo "Genesis:"
echo $genesis
echo $genesis | jq ". * {\"config\": $chainconfig}" > /genesis.json



echo "Before mapper.jq"
cat /genesis.json

# Configure and set the chain definition for the node
configoverride=`jq -f /mapper.jq /genesis.json`
echo ".*$configoverride">/tempscript.jq
mergedconfig=`jq -f /tempscript.jq /chainspec/test.json`
echo $mergedconfig>/chainspec/test.json
echo "Chainspec:"
cat /chainspec/test.json
# Load any keys explicitly added to the node
#if [ -d /keys ]; then
#	export NETHERMIND_HIVECONFIG_KEYSDIR=keys
#fi

echo "Running Nethermind..."
dotnet /nethermind/Nethermind.Runner.dll --config /configs/test.cfg  2>&1
