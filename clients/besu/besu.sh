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
#  - HIVE_FORK_LONDON          block number for London transition
#  - HIVE_FORK_SHANGHAI        block number for Shanghai transition
#  - HIVE_FORK_CANCUN          block number for Cancun transition
#  - HIVE_FORK_PRAGUE          block number for Prague transition
#  - HIVE_FORK_OSAKA           block number for Osaka transition
#  - HIVE_MINER                address to credit with mining rewards (single thread)
#  - HIVE_MINER_EXTRA          extra-data field to set for newly minted blocks
#  - HIVE_SKIP_POW             If set, skip PoW verification during block import
#  - HIVE_LOGLEVEL		         Simulator loglevel

# These flags are not supported by the Besu hive client
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork

besu=/opt/besu/bin/besu
RPCFLAGS="--graphql-http-enabled --graphql-http-host=0.0.0.0 --graphql-http-port=8545 --host-whitelist=*"
RPCFLAGS="$RPCFLAGS --rpc-http-enabled --rpc-http-api=ETH,NET,WEB3,ADMIN --rpc-http-host=0.0.0.0"

set -e

if [ "$HIVE_SKIP_POW" != "" ]; then
	IMPORTFLAGS="--skip-pow-validation-enabled"
fi
if [ "$HIVE_BOOTNODE" != "" ]; then
  FLAGS="$FLAGS --bootnodes=$HIVE_BOOTNODE"
fi

if [ "$HIVE_NETWORK_ID" != "" ]; then
	FLAGS="$FLAGS --network-id=$HIVE_NETWORK_ID"
else
    FLAGS="$FLAGS --network-id=1337"
fi

if [ "$HIVE_NODETYPE" == "full" ]; then
	FLAGS="$FLAGS --sync-mode=FAST --fast-sync-min-peers=1 --Xsynchronizer-fast-sync-pivot-distance=0"

fi
if [ "$HIVE_NODETYPE" == "light" ]; then
    echo "Besu does not support light nodes"
fi

# Override any chain configs fron ENV vars into json
chainconfig="{\"ethash\": {}}"
JQPARAMS=". "
if [ "$HIVE_CHAIN_ID" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"chainID\": $HIVE_CHAIN_ID}"
fi
if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"homesteadBlock\": $HIVE_FORK_HOMESTEAD}"
fi
# Besu requires forks to be in order. Hive tries to put the dao fork at block 2K when it's
# not activated, but besu rejects that
if [ "$HIVE_FORK_DAO_BLOCK" != "" ] && [ "$HIVE_FORK_DAO_BLOCK" != "2000" ]; then
  JQPARAMS="$JQPARAMS + {\"daoForkBlock\": $HIVE_FORK_DAO_BLOCK}"
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
  JQPARAMS="$JQPARAMS + {\"constantinopleFixBlock\": $HIVE_FORK_PETERSBURG}"
fi
if [ "$HIVE_FORK_ISTANBUL" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"istanbulBlock\": $HIVE_FORK_ISTANBUL}"
fi
if [ "$HIVE_FORK_MUIR_GLACIER" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"muirGlacierBlock\": $HIVE_FORK_MUIR_GLACIER}"
fi
if [ "$HIVE_FORK_BERLIN" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"berlinBlock\": $HIVE_FORK_MUIR_BERLIN}"
fi
if [ "$HIVE_FORK_LONDON" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"londonBlock\": $HIVE_FORK_MUIR_LONDON}"
fi
if [ "$HIVE_FORK_SHANGHAI" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"shanghaiBlock\": $HIVE_FORK_MUIR_SHANGHAI}"
fi
if [ "$HIVE_FORK_CANCUN" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"cancunBlock\": $HIVE_FORK_MUIR_CANCUN}"
fi
if [ "$HIVE_FORK_PRAGUE" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"pragueBlock\": $HIVE_FORK_MUIR_PRAGUE}"
fi
if [ "$HIVE_FORK_OSAKA" != "" ]; then
  JQPARAMS="$JQPARAMS + {\"osakaBlock\": $HIVE_FORK_MUIR_OSAKA}"
fi

chainconfig=`echo $chainconfig | jq "$JQPARAMS"`
cat /genesis.json | jq ". + {\"config\": $chainconfig}" > /besugenesis.json

if [ "$HIVE_MINER" != "" ]; then
  FLAGS="$FLAGS --miner-enabled --miner-coinbase=$HIVE_MINER"
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
  FLAGS="$FLAGS --miner-extra-data=$HIVE_MINER_EXTRA"
fi

if [ "$HIVE_LOGLEVEL" == "0" ]; then
  FLAGS="$FLAGS --logging=OFF"
elif [ "$HIVE_LOGLEVEL" == "1" ]; then
  FLAGS="$FLAGS --logging=ERROR"
elif [ "$HIVE_LOGLEVEL" == "2" ]; then
  FLAGS="$FLAGS --logging=WARN"
elif [ "$HIVE_LOGLEVEL" == "3" ]; then
  FLAGS="$FLAGS --logging=INFO"
elif [ "$HIVE_LOGLEVEL" == "4" ]; then
  FLAGS="$FLAGS --logging=DEBUG"
elif [ "$HIVE_LOGLEVEL" == "5" ]; then
  FLAGS="$FLAGS --logging=TRACE"
  env
  echo $FLAGS
  cat /besugenesis.json
fi
echo "Using the following genesis"
cat /besugenesis.json

FLAGS="$FLAGS --genesis-file=/besugenesis.json"
FLAGS="$FLAGS --min-gas-price=16 --tx-pool-price-bump=0"

# Allow import to fail
set +e
# Load the test chain if present
if [ -f /chain.rlp ]; then
	echo "Loading initial blockchain..."
	cmd="$besu $FLAGS blocks import $IMPORTFLAGS --from=/chain.rlp"
	echo "invoking $cmd"
	$cmd
fi


# Load the remainder of the test chain
if [ -d /blocks ]; then
        echo "Loading remaining individual blocks..."
  blocks=`ls /blocks | sort -n`
  home=`pwd`
  pushd /blocks
  cmd="$besu $FLAGS --data-path=$home blocks import $IMPORTFLAGS `ls /blocks | sort -n`"
  echo "in `pwd` invoking $cmd"
  $cmd
  popd
fi

set -e

# After block import, we also open for RPC
cmd="$besu $FLAGS $RPCFLAGS"
echo "starting main client: $cmd"
$cmd

