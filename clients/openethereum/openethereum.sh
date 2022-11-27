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
#
#  - HIVE_BOOTNODE             enode URL of the remote bootstrap node
#  - HIVE_NETWORK_ID           network ID number to use for the eth protocol
#  - HIVE_CHAIN_ID             network ID number to use for the eth protocol
#  - HIVE_NODETYPE             sync and pruning selector (archive, full, light)
#  - HIVE_SKIP_POW             if set, skip PoW verification during block import
#
# Forks:
#
#  - HIVE_FORK_HOMESTEAD       block number of the DAO hard-fork transition
#  - HIVE_FORK_DAO_BLOCK       block number of the DAO hard-fork transition
#  - HIVE_FORK_TANGERINE       block number of TangerineWhistle
#  - HIVE_FORK_SPURIOUS        block number of SpuriousDragon
#  - HIVE_FORK_BYZANTIUM       block number for Byzantium transition
#  - HIVE_FORK_CONSTANTINOPLE  block number for Constantinople transition
#  - HIVE_FORK_PETERSBURG      block number for ConstantinopleFix/PetersBurg transition
#  - HIVE_FORK_BERLIN          block number for Berlin transition
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
#  - HIVE_SKIP_POW             If set, skip PoW verification
#  - HIVE_LOGLEVEL             Client log level
#
# These variables are not supported by OpenEthereum:
#
#  - HIVE_FORK_DAO_VOTE        whether the node support (or opposes) the DAO fork
#  - HIVE_GRAPHQL_ENABLED      if set, GraphQL is enabled on port 8545
#  - HIVE_TESTNET              whether testnet nonces (2^20) are needed
#  - HIVE_ZERO_BASE_FEE        If set, remove base fee and permit zero gas network

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

# Disable PoW for import if set.
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
        echo "openethereum $FLAGS import /blocks/$block"
        $OE $FLAGS import /blocks/$block
    done
fi

# Immediately abort the script on any error encountered
set -e

# Configure p2p networking.
FLAGS="$FLAGS --nat none"
if [ "$HIVE_BOOTNODE" != "" ]; then
    FLAGS="$FLAGS --bootnodes $HIVE_BOOTNODE"
fi

# Import clique private key.
if [ "$HIVE_CLIQUE_PRIVATEKEY" != "" ]; then
    echo "Importing clique private key..."

    # OpenEthereum does not support importing raw private keys directly.
    # The private key needs to be turned into an encrypted keystore file first.
    echo "$HIVE_CLIQUE_PRIVATEKEY" > /tmp/clique-key
    echo "very secret" > /tmp/clique-key-password.txt
    /ethkey generate --lightkdf --privatekey /tmp/clique-key --passwordfile /tmp/clique-key-password.txt /tmp/clique-key.json

    # Now we can import it.
    $OE account import --chain /chain.json /tmp/clique-key.json

    # Set the password file flag so OE can decrypt the key later.
    FLAGS="$FLAGS --password=/tmp/clique-key-password.txt"
fi

# Configure mining.
if [ "$HIVE_MINER" != "" ]; then
    if [ "$HIVE_CLIQUE_PERIOD" != "" ]; then
        # Clique mining requested, set signer address.
        FLAGS="$FLAGS --engine-signer $HIVE_MINER --force-sealing"
    else
        # PoW mining requested, run ethminer.
        FLAGS="$FLAGS --author $HIVE_MINER"
        ethminer --mining-threads 1 &
    fi
fi
if [ "$HIVE_MINER_EXTRA" != "" ]; then
    FLAGS="$FLAGS --extra-data $HIVE_MINER_EXTRA"
fi
FLAGS="$FLAGS --min-gas-price=16000000000"

# Configure RPC.
FLAGS="$FLAGS --jsonrpc-interface all --jsonrpc-hosts all --jsonrpc-apis all --ws-origins all --ws-interface all"

# Disable eth price lookup.
FLAGS="$FLAGS --usd-per-eth 1"

# Run OpenEthereum!
echo "running $OE $FLAGS"
$OE $FLAGS
