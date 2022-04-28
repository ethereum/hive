#!/bin/sh

cd /hive/contracts
yarn
yarn build
export L2OO_STARTING_BLOCK_TIMESTAMP=$(cat /hive/genesis_timestamp)
echo "L2OO_STARTING_BLOCK_TIMESTAMP=$L2OO_STARTING_BLOCK_TIMESTAMP"
yarn hardhat --network devnetL1 deploy

echo "Deployed L1 contracts"
