#!/bin/sh
set -exu

function get_deployed_bytecode() {
    echo $(jq -r .deployedBytecode $1)
}

# Pull out the necessary bytecode/addresses from the artifacts/deployments.
L1_BLOCK_INFO_BYTECODE=$(get_deployed_bytecode /L1Block.json)
L2_TO_L1_MESSAGE_PASSER_BYTECODE=$(get_deployed_bytecode /L2ToL1MessagePasser.json)
L2_CROSS_DOMAIN_MESSENGER_BYTECODE=$(get_deployed_bytecode /L2CrossDomainMessenger.json)
OPTIMISM_MINTABLE_TOKEN_FACTORY_BYTECODE=$(get_deployed_bytecode /OptimismMintableTokenFactory.json)
L2_STANDARD_BRIDGE_BYTECODE=$(get_deployed_bytecode /L2StandardBridge.json)

GENESIS_TIMESTAMP=$(cat /genesis_timestamp)

## Replace values in the L2 genesis file.
## NOTE: these files are injected from devnet; see devnet.go
jq ". | .alloc.\"4200000000000000000000000000000000000015\".code = \"$L1_BLOCK_INFO_BYTECODE\"" < /genesis.json | \
  jq ". | .alloc.\"4200000000000000000000000000000000000015\".balance = \"0x0\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000016\".code = \"$L2_TO_L1_MESSAGE_PASSER_BYTECODE\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000016\".balance = \"0x0\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000007\".code = \"$L2_CROSS_DOMAIN_MESSENGER_BYTECODE\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000007\".balance = \"0x0\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000012\".code = \"$OPTIMISM_MINTABLE_TOKEN_FACTORY_BYTECODE\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000012\".balance = \"0x0\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000010\".code = \"$L2_STANDARD_BRIDGE_BYTECODE\"" | \
  jq ". | .alloc.\"4200000000000000000000000000000000000010\".balance = \"0x0\"" | \
  jq ". | .timestamp = \"$GENESIS_TIMESTAMP\" " > /hive/genesis-l2.json

VERBOSITY=${GETH_VERBOSITY:-3}
GETH_DATA_DIR=/db
GETH_CHAINDATA_DIR="$GETH_DATA_DIR/geth/chaindata"
GETH_KEYSTORE_DIR="$GETH_DATA_DIR/keystore"
CHAIN_ID=$(cat /hive/genesis-l2.json | jq -r .config.chainId)
BLOCK_SIGNER_PRIVATE_KEY="3e4bde571b86929bf08e2aaad9a6a1882664cd5e65b96fff7d03e1c4e6dfa15c"
BLOCK_SIGNER_ADDRESS="0xca062b0fd91172d89bcd4bb084ac4e21972cc467"

if [ ! -d "$GETH_KEYSTORE_DIR" ]; then
	echo "$GETH_KEYSTORE_DIR missing, running account import"
	echo -n "pwd" > "$GETH_DATA_DIR"/password
	echo -n "$BLOCK_SIGNER_PRIVATE_KEY" | sed 's/0x//' > "$GETH_DATA_DIR"/block-signer-key
	geth account import \
		--datadir="$GETH_DATA_DIR" \
		--password="$GETH_DATA_DIR"/password \
		"$GETH_DATA_DIR"/block-signer-key
else
	echo "$GETH_KEYSTORE_DIR exists."
fi

if [ ! -d "$GETH_CHAINDATA_DIR" ]; then
	echo "$GETH_CHAINDATA_DIR missing, running init"
	echo "Initializing genesis."
	geth --verbosity="$VERBOSITY" init \
		--datadir="$GETH_DATA_DIR" \
		"/hive/genesis-l2.json"
else
	echo "$GETH_CHAINDATA_DIR exists."
fi

# Warning: Archive mode is required, otherwise old trie nodes will be
# pruned within minutes of starting the devnet.

exec geth \
	--datadir="$GETH_DATA_DIR" \
	--verbosity="$VERBOSITY" \
	--http \
	--http.corsdomain="*" \
	--http.vhosts="*" \
	--http.addr=0.0.0.0 \
	--http.port=9545 \
	--http.api=web3,debug,eth,txpool,net,engine \
	--ws \
	--ws.addr=0.0.0.0 \
	--ws.port=9546 \
	--ws.origins="*" \
	--ws.api=debug,eth,txpool,net,engine \
	--syncmode=full \
	--nodiscover \
	--maxpeers=1 \
	--networkid=$CHAIN_ID \
	--unlock=$BLOCK_SIGNER_ADDRESS \
	--mine \
	--miner.etherbase=$BLOCK_SIGNER_ADDRESS \
	--password="$GETH_DATA_DIR"/password \
	--allow-insecure-unlock \
	--gcmode=archive \
	"$@"
