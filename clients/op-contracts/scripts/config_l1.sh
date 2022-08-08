#!/bin/bash

set -eu

cd /hive/optimism/packages/contracts-bedrock

# generate L1 config, redirect standard output to stderr, we output the result as json to stdout later
npx hardhat --network hivenet genesis-l1 --outfile genesis-l1.json 1>&2

cat genesis-l1.json
