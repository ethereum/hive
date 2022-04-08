#!/bin/bash

# Script to retrieve the enode
# 
# This is copied into the validator container by Hive
# and used to provide a client-specific enode id retriever
#

# Immediately abort the script on any error encountered
set -e

data='{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'
TARGET_RESPONSE=$(curl -s -X POST  -H "Content-Type: application/json"  --data $data "localhost:8545" )
TARGET_ENODE=$(echo ${TARGET_RESPONSE}| jq -r '.result.enode')
echo "$TARGET_ENODE"