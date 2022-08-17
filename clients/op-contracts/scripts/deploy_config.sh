#!/bin/bash

set -eu

cd /opt/optimism/packages/contracts-bedrock

# Deploy config is provided as first script argument
echo "$1" > deploy-config/hivenet.json
