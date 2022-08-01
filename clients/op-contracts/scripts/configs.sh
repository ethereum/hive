#!/bin/bash

set -eu

cd /hive/optimism/packages/contracts-bedrock

# Deploy config is provided as first script argument
echo "$1" >> deploy-config/hivenet_data.json

# Our hardhat system wants a typescript deploy config, not json. Just wrap the json.
cat > deploy-config/hivenet.ts << EOF
import hivenetData from "hivenet_data.json"

const config = hivenetData;

export default config
EOF

# generate configs, redirect standard output to stderr, we output the results as a merged json to stdout later

npx hardhat --network hivenet genesis-l1 --outfile genesis-l1.json 1>&2

npx hardhat --network hivenet genesis-l2 --outfile genesis-l2.json 1>&2

npx hardhat--network hivenet rollup-config --outfile rollup.json 1>&2

jq '{"l1": input[0], "l2": input[1], "rollup": input[2]} . ' genesis-l1.json genesis-l2.json rollup.json
