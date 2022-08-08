#!/bin/bash

echo "starting op-contracts"

# this web-browser will keep ensure the container is considered ready and alive by Hive,
# and make it easy to navigate the live container contents.
http-server -p 8545 -d -i -a 0.0.0.0 /hive/optimism > /dev/null
