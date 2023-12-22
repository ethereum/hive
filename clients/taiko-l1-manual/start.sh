#!/bin/sh

set -e

CHAIN_ID=31336
PERIOD=12
NODE_KEY="9c52ecfdf0397641379694b367a2bf4328580e4310753d4064ab0af8b132d565"

echo "CHAIN_ID: $CHAIN_ID"
echo "PERIOD: $PERIOD"
echo "NODE_KEY: $NODE_KEY"

cat /host/l1_genesis.json | sed -i "s/CHAIN_ID_PLACE_HOLDER/$CHAIN_ID/g" /host/l1_genesis.json
cat /host/l1_genesis.json | sed -i "s/PERIOD_PLACE_HOLDER/$PERIOD/g" /host/l1_genesis.json

geth init --datadir /data/l1-node /host/l1_genesis.json

geth --datadir /data/l1-node \
  --nodiscover \
  --allow-insecure-unlock \
  --verbosity 2 \
  --rpc.enabledeprecatedpersonal \
  --exec 'personal.importRawKey("'ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80'", null)' console

geth --datadir /data/l1-node \
  --gcmode archive \
  --networkid $CHAIN_ID \
  --http \
  --http.addr 0.0.0.0 \
  --http.vhosts=* \
  --http.corsdomain '*' \
  --http.api admin,debug,eth,net,web3,txpool,miner \
  --ws \
  --ws.addr 0.0.0.0 \
  --ws.origins '*' \
  --ws.api admin,debug,eth,net,web3,txpool,miner \
  --allow-insecure-unlock \
  --password /dev/null \
  --unlock 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266 \
  --nodekeyhex $NODE_KEY \
  --log.json \
  --mine \
  --miner.etherbase "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266" \
  --nat none

