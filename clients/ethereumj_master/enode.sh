#!/bin/bash

# Script to retrieve the enode
# 
# This is copied into the validator container by Hive
# and used to provide a client-specific enode id retriever
#

# Immediately abort the script on any error encountered
set -e

#TODO How to get enode id from Ethereumj

#TARGET_ENODE=$(curl -s -X POST --data '{"jsonrpc":"2.0","method":"admin_nodeInfo"","params":[],"id":1}' "$HIVE_CLIENT_IP:8545" | jq -r '.enode')
#echo "Target enode identified as $TARGET_ENODE"