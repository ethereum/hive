#!/bin/bash

# Script to retrieve the ENR

# Immediately abort the script on any error encountered
set -e


TARGET_RESPONSE=$(curl -s -X POST  -H "Content-Type: application/json" "localhost:8545/eth/v1/node/identity" )


TARGET_ENR=$(echo ${TARGET_RESPONSE}| jq -r '.data.enr')
echo "$TARGET_ENR"
