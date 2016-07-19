#!/bin/sh
set -e

# Define the "Hello, World" RLP encoded string
expect="0x0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000000d48656c6c6f2c20576f726c642100000000000000000000000000000000000000"

# Try to execute a call to the greeter and verify the result is "Hello, World!"
params='[{"from": "0x626f1e3715807da7747951690fbead49d6ae6b25", "to": "0x3d4ae584ec09c11ee48a22b2850652cb976d6613", "data": "0x5ae5d4aa"}, "latest"]'
result=`curl -s -X POST --data "{\"jsonrpc\":\"2.0\",\"method\":\"eth_call\",\"params\":$params,\"id\":0}" "$HIVE_CLIENT_IP:8545" | jq '.result' | tr -d '"'`

if [ "$result" != "$expect" ]; then
	echo "Call result mismatch: have $result, want $expect"
	exit 1
fi
