#!/bin/sh

# Immediately abort the script on any error encountered
set -e

sed -i "s/mixHash/mixhash/" genesis.json

if [ $HIVE_TESTNET ];
then
  # TODO: Write better sed magic?
  sed -i "s/  ACCOUNT_INITIAL_NONCE.*/  ACCOUNT_INITIAL_NONCE: 1048576/g" /root/.config/pyethapp/config.yaml
fi

if [ -f /chain.rlp ]; then
  echo "Found chain.rlp"
  pyethapp import /chain.rlp
  if [ -d /blocks/ ]; then
    # VAST HACK! Turns out that trying to get a functioning pyethapp environment in order to import blocks
    # without actually running pyethapp is its own special form of torment. So...
    echo "from importblock import Importer; Importer(eth).run()" | pyethapp run --console
  fi
fi

if [ -d /keys/ ]; then
  cp -r /keys/ /root/.config/pyethapp/keystore
fi

# --dev stops on error.

pyethapp run --dev

