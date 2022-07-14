#!/bin/bash

# Script to retrieve the enode
# 
# This is copied into the validator container by Hive
# and used to provide a client-specific enode id retriever
#

# Immediately abort the script on any error encountered
set -e

data='{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}'
while [[ "$TARGET_RESPONSE" != *enode* ]]
do
    TARGET_RESPONSE=$(curl -s -X POST  -H "Content-Type: application/json"  --data $data "localhost:8545" )
    ((c++)) && ((c==50)) && break
    sleep 0.1
done

TARGET_ENODE=$(echo ${TARGET_RESPONSE}| jq -r '.result.enode')
echo "$TARGET_ENODE"
#echo "$TARGET_RESPONSE"
