#!/bin/sh

cd /hive/contracts
echo "node version"
node --version
yarn
yarn build
export L2OO_STARTING_BLOCK_TIMESTAMP=$(cat /hive/genesis_timestamp)
echo "L2OO_STARTING_BLOCK_TIMESTAMP=$L2OO_STARTING_BLOCK_TIMESTAMP"
forge --version
yarn hardhat --network devnetL1 deploy 2>&1

echo "Deployed L1 contracts"
