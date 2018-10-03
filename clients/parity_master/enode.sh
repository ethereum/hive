#!/bin/bash

# Script to retrieve the enode
# 
# This is copied into the validator container by Hive
# and used to provide a client-specific enode id retriever
#

# Immediately abort the script on any error encountered
set -e




TARGET_RESPONSE=$(curl --data '{"method":"parity_enode","params":[],"id":1,"jsonrpc":"2.0"}' -H "Content-Type: application/json" -X POST "localhost:8545" )


TARGET_ENODE=$(echo ${TARGET_RESPONSE}| jq -r '.result')

echo "$TARGET_ENODE"