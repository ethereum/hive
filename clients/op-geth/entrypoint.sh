#!/bin/sh
set -exu

# note: geth wants an integer log level (see L1 hive definition)
VERBOSITY=${HIVE_ETH1_LOGLEVEL:-3}

GETH_DATA_DIR=/db
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"

CHAIN_ID=$(cat /genesis.json | jq -r .config.chainId)

if [ ! -d "$GETH_CHAINDATA_DIR" ]; then
	echo "$GETH_CHAINDATA_DIR missing, running init"
	echo "Initializing genesis."
	geth --verbosity="$VERBOSITY" init \
		--datadir="$GETH_DATA_DIR" \
		"/genesis.json"
else
	echo "$GETH_CHAINDATA_DIR exists."
fi

# Warning: Archive mode is required, otherwise old trie nodes will be
# pruned within minutes of starting the devnet.

# TODO: increase max peers, enable p2p, once we get to snap sync testing in Hive.

geth \
	--datadir="$GETH_DATA_DIR" \
	--verbosity="$VERBOSITY" \
	--http \
	--http.corsdomain="*" \
	--http.vhosts="*" \
	--http.addr=0.0.0.0 \
	--http.port=8545 \
	--http.api=web3,debug,eth,txpool,net,engine \
	--ws \
	--ws.addr=0.0.0.0 \
	--ws.port=8546 \
	--ws.origins="*" \
	--ws.api=debug,eth,txpool,net,engine \
	--authrpc.jwtsecret="/hive/input/jwt-secret.txt" \
	--authrpc.port=8551 \
	--authrpc.addr=0.0.0.0 \
	--syncmode=full \
	--nodiscover \
	--maxpeers=0 \
	--networkid="$CHAIN_ID" \
	--mine \
	--miner.etherbase="$HIVE_ETHERBASE" \
	--password="$GETH_DATA_DIR"/password \
	--allow-insecure-unlock \
	--gcmode=archive \
	"$@"
