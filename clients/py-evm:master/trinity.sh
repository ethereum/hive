#!/bin/bash

# Startup script to initialize and boot a go-ethereum instance.
#
# This script assumes the following files:
#  - `trinity` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE       enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID     network ID number to use for the eth protocol
#  - HIVE_TESTNET        whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE       sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_VOTE  whether the node support (or opposes) the DAO fork
#  - HIVE_FORK_TANGERINE block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS  block number of SpuriousDragon
#  - HIVE_FORK_METROPOLIS block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE block number for Constantinople transition
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks

# Immediately abort the script on any error encountered
set -e

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --preferred-node=$HIVE_BOOTNODE"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id=$HIVE_NETWORK_ID"
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS --ropsten"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS --sync-mode=full"
fi
if [ "$HIVE_NODETYPE" == "light" ]; then
	FLAGS="$FLAGS --light"
fi

# Override any chain configs in the go-ethereum specific way
chainconfig="{}"
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"homesteadBlock\": $HIVE_FORK_HOMESTEAD}"`
fi
if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkBlock\": $HIVE_FORK_DAO_BLOCK}"`
fi
if [ "$HIVE_FORK_DAO_VOTE" == "0" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkSupport\": false}"`
fi
if [ "$HIVE_FORK_DAO_VOTE" == "1" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"daoForkSupport\": true}"`
fi

if [ "$HIVE_FORK_TANGERINE" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"eip150Block\": $HIVE_FORK_TANGERINE}"`
fi
if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"eip158Block\": $HIVE_FORK_SPURIOUS}"`
	chainconfig=`echo $chainconfig | jq ". + {\"eip155Block\": $HIVE_FORK_SPURIOUS}"`
fi
if [ "$HIVE_FORK_METROPOLIS" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"byzantiumBlock\": $HIVE_FORK_METROPOLIS}"`
fi
if [ "$HIVE_FORK_CONSTANTINOPLE" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"constantinopleBlock\": $HIVE_FORK_CONSTANTINOPLE}"`
fi

genesis=`cat /genesis.json` && echo $genesis | jq ". + {\"config\": $chainconfig}"

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
	genesis=`echo $genesis` | jq ". + {\"coinbase\": $HIVE_MINER}"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	genesis=`echo $genesis` | jq ". + {\"extraData\": $HIVE_MINER_EXTRA}"
fi

genesis=`echo $genesis` | jq ". + {\"config\": $chainconfig}" > /genesis.json

# set the genesis config flag
FLAGS="$FLAGS --genesis /genesis.json"

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	BLOCKS="/chain.rlp"

	echo "Loading remaining individual blocks..."
	if [ -d /blocks ]; then
		BLOCKs="$BLOCKS `ls | sort -n`"
	fi
	FLAGS="$FLAGS --import $BLOCKS"
fi

set -e

# Load any keys explicitly added to the node
if [ -d /keys ]; then
	# trinity does not currently support keystore flags
fi

# Run the py-evm implementation with the requested flags
echo "Running trinity..."
/trinity $FLAGS
