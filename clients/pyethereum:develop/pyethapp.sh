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
fi

if [ -d /blocks/ ]; then
  # VAST HACK! Turns out that trying to get a functioning pyethapp environment in order to import blocks
  # without actually running pyethapp is its own special form of torment. So...
  echo "from importblock import Importer; Importer(eth).run()" | pyethapp run --console
fi

if [ -d /keys/ ]; then
  cp -r /keys/ /root/.config/pyethapp/keystore
fi

# --dev stops on error.

pyethapp -c eth.genesis='{ "coinbase" : "0x8888f1f195afa192cfee860698584c030f4c9db1", "difficulty" : "0x020000", "extraData"  : "0x42", "gasLimit" : "0x2fefd8", "mixhash" : "0x2c85bcbce56429100b2108254bb56906257582aeafcbd682bc9af67a9f5aee46", "nonce" : "0x78cc16f7b4f65485", "parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000", "timestamp" : "0x54c98c81", "alloc" : { "a94f5374fce5edbc8e2a8697c15331677e6ebf0b": { "balance" : "0x09184e72a000" } } }' run --dev
