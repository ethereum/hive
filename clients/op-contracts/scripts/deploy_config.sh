#!/bin/bash

set -eu

cd /hive/optimism/packages/contracts-bedrock

# Deploy config is provided as first script argument
echo "$1" > deploy-config/hivenet_data.json
