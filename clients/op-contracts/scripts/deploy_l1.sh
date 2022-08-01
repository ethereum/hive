#!/bin/sh

set -e

yarn global add npx -W 2>&1
cd /hive/optimism
yarn 2>&1
yarn build 2>&1
cd /hive/optimism/packages/contracts-bedrock

forge --version
yarn hardhat --network hivenet deploy 2>&1

echo "Deployed L1 contracts"
