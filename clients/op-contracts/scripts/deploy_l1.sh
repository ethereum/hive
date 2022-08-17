#!/bin/sh

set -e

export L1_RPC="$1"
echo "configuring rollup config using L1 RPC: $L1_RPC" 1>&2

export PRIVATE_KEY_DEPLOYER="$2"
export CHAIN_ID="$3"

cd /opt/optimism/packages/contracts-bedrock

forge --version
yarn hardhat --network hivenet deploy 2>&1

echo "Deployed L1 contracts"
