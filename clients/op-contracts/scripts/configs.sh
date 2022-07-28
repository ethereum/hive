#!/bin/bash

set -e

# first and only argument is the timestamp
export L2OO_STARTING_BLOCK_TIMESTAMP=$1

# TODO: create custom hivenet hardhat deployment instead of using the devnet network.
cat > deploy-config/hivenet.ts << EOF
config stuff here
EOF

# generate configs, redirect standard output to stderr, we output the results as a merged json to stdout later

npx hardhat --network hivenet genesis-l1 --outfile genesis-l1.json 1>&2

npx hardhat --network hivenet genesis-l2 --outfile genesis-l2.json 1>&2

npx hardhat--network hivenet rollup-config --outfile rollup.json 1>&2

jq '{"l1": input[0], "l2": input[1], "rollup": input[2]} . ' genesis-l1.json genesis-l2.json rollup.json
