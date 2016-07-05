#!/bin/sh

# Startup script to initialize and boot a go-ethereum instance.
#
# This script assumes:
#  - `geth` binary is located in the filesystem root
#  - `genesis.json` file is located in the filesystem root
#  - `chain.rlp` file is located in the filesystem root

# Immediately abort the script on any error encountered
set -e

# Initialize the local testchain with the genesis state
echo "Initializing database with genesis state..."
/geth init /genesis.json
echo

# Load the test chain if present
echo "Loading initial blockchain..."
/geth import /chain.rlp
echo

# Load the remainder of the test chain
echo "Loading remaining individual blocks..."
for block in `ls /blocks | sort -n`; do
	/geth import /blocks/$block
done
echo

# Run the go-ethereum implementation with the requested flags
echo "Running go-ethereum..."
/geth --rpc --rpcaddr "0.0.0.0"
echo
