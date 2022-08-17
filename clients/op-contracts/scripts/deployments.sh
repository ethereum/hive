#!/bin/sh

set -e

cd /opt/optimism/packages/contracts-bedrock/deployments/hivenet/
# This combines all deployment objects into a single json object, keyed by contract name (file name stripped from .json)
jq -n 'reduce inputs as $s (.; .[input_filename|rtrimstr(".json")|ltrimstr("./")] += $s)' ./* || true
