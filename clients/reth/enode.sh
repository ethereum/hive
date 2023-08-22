#!/bin/bash

set -e

TARGET_RESPONSE=$(curl -s -X POST  -H "Content-Type: application/json"  --data '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' "127.0.0.1:8545" )
TARGET_ENODE=$(echo ${TARGET_RESPONSE}| jq -r '.result.enode')

echo "$TARGET_ENODE"
