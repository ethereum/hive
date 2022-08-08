#!/bin/bash

set -eu

cd /hive/optimism/packages/contracts-bedrock

# we use the L1 RPC to fetch L1 block info in hardhat
export L1_RPC="$1"
echo "configuring rollup config using L1 RPC: $L1_RPC" 1>&2

# we use the L2 RPC to fetch L2 block info in hardhat
export L2_RPC="$2"
echo "configuring rollup config using L2 RPC: $L2_RPC" 1>&2

# required for hardhat network definition to work with RPC
export PRIVATE_KEY_DEPLOYER="$3"
export CHAIN_ID="$4"

# redirect standard output to stderr, we output the result as json to stdout later
npx hardhat --network hivenet rollup-config --l1-rpc-url "$L1_RPC" --l2-rpc-url "$L2_RPC" --outfile rollup.json 1>&2

cat rollup.json
