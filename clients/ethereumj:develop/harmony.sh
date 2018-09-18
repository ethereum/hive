#!/bin/bash

# Startup script to initialize and boot a peer instance.
#
# This script assumes the following files:
#  - `ethereum-harmony` folder is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root (mandatory)
#  - `chain.rlp` file is located in the filesystem root (optional)
#  - `blocks` folder is located in the filesystem root (optional)
#  - `keys` folder is located in the filesystem root (optional)
#
# This script assumes the following environment variables:
#  - HIVE_BOOTNODE       enode URL of the remote bootstrap node
#  - HIVE_TESTNET        whether testnet nonces (2^20) are needed
#  - HIVE_NODETYPE       sync and pruning selector (archive, full, light)
#  - HIVE_FORK_HOMESTEAD block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_VOTE  whether the node support (or opposes) the DAO fork
#  - HIVE_FORK_TANGERINE block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS  block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM block number for Byzantium transition
#  - HIVE_MINER          address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA    extra-data field to set for newly minted blocks

# Immediately abort the script on any error encountered
set -e

# Runs Harmony client with specified options.
# Parameters:
#   $1 - run description;
#   $2 - Harmony options;
run_harmony() {
    echo "Harmony running: $1"
    echo "Harmony run options: $2"

    export HARMONY_ETHER_CAMP_OPTS=$2
    /harmony.ether.camp/bin/harmony.ether.camp
}

# It doesn't make sense to dial out, use only a pre-set bootnode
if [ "$HIVE_BOOTNODE" != "" ]; then
	FLAGS="$FLAGS -Dpeer.discovery.ip.list.0=$HIVE_BOOTNODE"
else
	FLAGS="$FLAGS"
	#FLAGS="$FLAGS -Dpeer.discovery.enabled=false"
fi

# If the client is to be run in testnet mode, flag it as such
if [ "$HIVE_TESTNET" == "1" ]; then
	FLAGS="$FLAGS -Dblockchain.config.name=ropsten"
fi

# Handle any client mode or operation requests
if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS -Dsync.fast.enabled=true"
fi
if [ "$HIVE_NODETYPE" == "light" ]; then
    echo "Missing --light implementation"
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
if [ "$HIVE_FORK_BYZANTIUM" != "" ]; then
	chainconfig=`echo $chainconfig | jq ". + {\"byzantiumBlock\": $HIVE_FORK_BYZANTIUM}"`
fi
if [ "$chainconfig" != "{}" ]; then
	genesis=`cat /genesis.json` && echo $genesis | jq ". + {\"config\": $chainconfig}" > /genesis.json
fi

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
FLAGS="$FLAGS -DgenesisFile=/genesis.json"
FLAGS="$FLAGS -Dethash.dir=/root/.ethash"

FLAGS="$FLAGS -Dserver.port=8545"
FLAGS="$FLAGS -Ddatabase.dir=database"
FLAGS="$FLAGS -Dlogs.keepStdOut=true"

FLAGS="$FLAGS -Dmodules.contracts.enabled=false"
FLAGS="$FLAGS -Dmodules.web.enabled=false"
FLAGS="$FLAGS -Dmodules.rpc.enabled=true"

FLAGS="$FLAGS -Dlogging.level.sync=ERROR"
FLAGS="$FLAGS -Dlogging.level.net=INFO"
FLAGS="$FLAGS -Dlogging.level.discover=ERROR"
FLAGS="$FLAGS -Dlogging.level.general=ERROR"
FLAGS="$FLAGS -Dlogging.level.mine=ERROR"
FLAGS="$FLAGS -Dlogging.level.jsonrpc=ERROR"
FLAGS="$FLAGS -Dlogging.level.harmony=ERROR"

# Load the test chain if present
echo "Loading initial blockchain..."
if [ -f /chain.rlp ]; then
    run_harmony "single blocks dump loading" "$FLAGS -Dblocks.format=rlp -Dblocks.loader=/chain.rlp -Dpeer.listen.port=0"
fi

# Load the remainder of the test chain
if [ -d /blocks ]; then
    run_harmony "multiple blocks dump loading" "$FLAGS -Dblocks.format=rlp -Dblocks.loader=/blocks -Dpeer.listen.port=0"
fi

# Load any keys explicitly added to the node
if [ -d /keys ]; then
    FLAGS="$FLAGS -Dkeystore.dir=/keys"
fi

# Configure any mining operation
if [ "$HIVE_MINER" != "" ]; then
    # We don't start mining here, but only set mining options
	FLAGS="$FLAGS -Dmine.coinbase=$HIVE_MINER -Dmine.cpuMineThreads=1"

	if [ "$HIVE_MINER_NO_AUTO_MINE" != "1" ]; then
        FLAGS="$FLAGS -Dmine.start=true -DnetworkProfile=private"
	fi
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
	FLAGS="$FLAGS -Dmine.extraData=$HIVE_MINER_EXTRA"
fi

# Run the peer implementation with the requested flags
run_harmony "Develop ..." "$FLAGS"
