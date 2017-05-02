#!/bin/bash

# Immediately abort the script on any error encountered
set -e

# Must delete a bunch of keys because pyethapp returns an error if any are present.
# Must also rename mixHash to mixhash, again, to please pyethapp.
genesis=$(cat /genesis.json | \
  jq 'del(.receiptTrie,.hash,.uncleHash,.number,.stateRoot,.transactionsTrie,.bloom,.gasUsed) | with_entries( if .key == "mixHash" then .key |= sub("H";"h") else . end)')

# Don't immediately abort, some imports are meant to fail
set +e

if [ -f /chain.rlp ]; then
  echo "Found chain.rlp"
  pyethapp import /chain.rlp
fi

if [ "$HIVE_FORK_METROPOLIS" != "" ]; then
  extra_config="-c eth.block.METROPOLIS_FORK_BLKNUM=$HIVE_FORK_METROPOLIS"
fi

if [ "$HIVE_FORK_HOMESTEAD" != "" ]; then
  extra_config="$extra_config -c eth.block.HOMESTEAD_FORK_BLKNUM=$HIVE_FORK_HOMESTEAD"
fi

if [ "$HIVE_FORK_TANGERINE" != "" ]; then
  extra_config="$extra_config -c eth.block.ANTI_DOS_FORK_BLKNUM=$HIVE_FORK_TANGERINE"
fi

if [ "$HIVE_FORK_SPURIOUS" != "" ]; then
  extra_config="$extra_config -c eth.block.CLEARING_FORK_BLKNUM=$HIVE_FORK_SPURIOUS"
fi

if [ "$HIVE_FORK_DAO_BLOCK" != "" ]; then
  extra_config="$extra_config -c eth.block.DAO_FORK_BLKNUM=$HIVE_FORK_DAO_BLOCK"
fi

if [ -d /blocks/ ]; then
  # VAST HACK! Turns out that trying to get a functioning pyethapp environment in order to import blocks
  # without actually running pyethapp is its own special form of torment. So...
  echo "from importblock import Importer; Importer(eth).run()" | \
    pyethapp -c eth.genesis="$genesis" $extra_config run --console
fi

set -e

if [ -d /keys/ ]; then
  cp -r /keys/ /root/.config/pyethapp/keystore
fi

# --dev stops on error.

pyethapp -c eth.genesis="$genesis" $extra_config run --dev
