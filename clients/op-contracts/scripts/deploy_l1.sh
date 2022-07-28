#!/bin/sh

set -e

yarn global add npx -W 2>&1
cd /hive/optimism
yarn 2>&1
yarn build 2>&1
cd /hive/optimism/packages/contracts-bedrock

export L2OO_STARTING_BLOCK_TIMESTAMP=$(cat /hive/genesis_timestamp)
echo "L2OO_STARTING_BLOCK_TIMESTAMP=$L2OO_STARTING_BLOCK_TIMESTAMP"
forge --version
yarn hardhat --network devnetL1 deploy 2>&1

echo "Deployed L1 contracts"
