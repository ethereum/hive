#!/bin/sh
set -e

# Simply query the account list and check that everything's present
reply=`curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_accounts","params":[],"id":1}' "$HIVE_CLIENT_IP:8545"`
if [ "`echo $reply | grep -F '0x089dab00d3a33707260eae6913419c7f3bc572b2' | grep -F '0x705cc3e6cd8da803933cfc89bb0399faff05e4a3'`" == "" ]; then
	echo "Mismatching keys: $reply"
	exit 1
fi
