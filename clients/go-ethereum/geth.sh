#!/bin/bash

# Startup script to initialize and boot a go-ethereum instance.
#
# This script assumes the following files:
#  - `geth` binary is located in the filesystem root
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
#  - HIVE_FORK_BYZANTIUM block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG  block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW       If set, skip PoW verification during block import
#  - HIVE_LOGLEVEL		 Simulator loglevel

# Immediately abort the script on any error encountered
set -e

#It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
else
	FLAGS="$FLAGS --nodiscover"
fi

if [ "$HIVE_SKIP_POW" != "" ]; then
	FLAGS="$FLAGS --fakepow"
fi

# If a specific network ID is requested, use that
if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --networkid $HIVE_NETWORK_ID"
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS --testnet"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS --syncmode fast "
fi
if [ "$HIVE_NODETYPE" == "light" ]; then
	FLAGS="$FLAGS --syncmode light "
fi






if [ "$HIVE_USE_GENESIS_CONFIG" == "" ]; then

	# Override any chain configs in the go-ethereum specific way
	chainconfig="{}"
	JQPARAMS=". "
	if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"homesteadBlock\": $HIVE_FORK_HOMESTEAD}"
	fi
	if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"daoForkBlock\": $HIVE_FORK_DAO_BLOCK}"
	fi
	if [ "$HIVE_FORK_DAO_VOTE" == "0" ]; then
		JQPARAMS="$JQPARAMS + {\"daoForkSupport\": false}"
	fi
	if [ "$HIVE_FORK_DAO_VOTE" == "1" ]; then
		JQPARAMS="$JQPARAMS + {\"daoForkSupport\": true}"
	fi

	if [ "$HIVE_FORK_TANGERINE" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"eip150Block\": $HIVE_FORK_TANGERINE}"
	fi
	if [ "$HIVE_FORK_TANGERINE_HASH" != "" ]; then
		chainconfig=`echo $chainconfig | jq ". + {\"eip150Hash\": $HIVE_FORK_TANGERINE_HASH}"`
	fi
	if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"eip158Block\": $HIVE_FORK_SPURIOUS}"
		JQPARAMS="$JQPARAMS + {\"eip155Block\": $HIVE_FORK_SPURIOUS}"
	fi
	if [ "$HIVE_FORK_BYZANTIUM" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"byzantiumBlock\": $HIVE_FORK_BYZANTIUM}"
	fi
	if [ "$HIVE_FORK_CONSTANTINOPLE" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"constantinopleBlock\": $HIVE_FORK_CONSTANTINOPLE}"
	fi
	if [ "$HIVE_FORK_PETERSBURG" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"petersburgBlock\": $HIVE_FORK_PETERSBURG}"
	fi
	if [ "$HIVE_FORK_ISTANBUL" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"istanbulBlock\": $HIVE_FORK_ISTANBUL}"
	fi
	if [ "$HIVE_CHAIN_ID" != "" ]; then
		JQPARAMS="$JQPARAMS + {\"chainId\": $HIVE_CHAIN_ID}"
	fi
	chainconfig=`echo $chainconfig | jq "$JQPARAMS"`
	genesis=`cat /genesis.json` && echo $genesis | jq ". + {\"config\": $chainconfig}" > /genesis.json
fi

# Dump genesis
echo "Supplied genesis state:"
cat /genesis.json

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
/geth $FLAGS init /genesis.json

# Don't immediately abort, some imports are meant to fail
set +e

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
	/geth $FLAGS --gcmode=archive import /chain.rlp
else
	echo "Warning: chain.rlp not found."
fi

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
if [ -d /blocks ]; then
	(cd blocks && ../geth $FLAGS --gcmode=archive --verbosity=$HIVE_LOGLEVEL --nocompaction import `ls | sort -n`)
else
	echo "Warning: blocks folder not found."
fi

set -e

# Load any keys explicitly added to the node
if [ -d /keys ]; then
	FLAGS="$FLAGS --keystore /keys"
fi

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
	FLAGS="$FLAGS --mine --minerthreads 1 --etherbase $HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS --extradata $HIVE_MINER_EXTRA"
fi

# Run the go-ethereum implementation with the requested flags

echo "Running go-ethereum with flags $FLAGS"
/geth $FLAGS  --verbosity=$HIVE_LOGLEVEL --nat=none --rpc --rpcaddr "0.0.0.0" --rpcapi "admin,debug,eth,miner,net,personal,shh,txpool,web3" --ws --wsaddr "0.0.0.0" --wsapi "admin,debug,eth,miner,net,personal,shh,txpool,web3" --wsorigins "*"

