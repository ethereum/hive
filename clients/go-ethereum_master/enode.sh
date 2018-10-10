#!/bin/bash

# Script to retrieve the enode
# 
# This is copied into the validator container by Hive
# and used to provide a client-specific enode id retriever
#

# Immediately abort the script on any error encountered
set -e
echo "Trying to get enode."
TARGET_RESPONSE= `curl -s -X POST --data '{"jsonrpc":"2.0","method":"admin_nodeInfo"","params":[],"id":1}' "$HIVE_CLIENT_IP:8545" `
echo "Got admin enode info response: $TARGET_RESPONSE"
TARGET_ENODE=`echo ${TARGET_RESPONSE}| jq -r '.enode'`
echo "Target enode identified as $TARGET_ENODE"