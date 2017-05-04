#!/bin/sh
set -e

# Query the genesis block and check that it's correct (hash requires correct nonces)
reply=`curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":1}' "$HIVE_CLIENT_IP:8545"`

hash=`echo $reply | sed 's/,/\n/g' | grep hash | cut -d ':' -f 2 | tr -d '"'`
if [ $hash != "0x0cd786a2425d16f152c658316c423e6ce1181e15c3295826d7c9904cba9ce303" ]; then
	echo "Hash mismatch: have $hash, want 0x0cd786a2425d16f152c658316c423e6ce1181e15c3295826d7c9904cba9ce303"
	exit 1
fi

# Query the nonce of an existent account
reply=`curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0x102e61f5d8f9bc71d0ad4a084df4e65e05ce0e1c", "latest"],"id":1}' "$HIVE_CLIENT_IP:8545"`

nonce=`echo $reply | sed 's/,/\n/g' | sed 's/}/\n/g' | grep result | cut -d ':' -f 2 | tr -d '"'`
if [ $nonce != "0x100000" ]; then
        echo $reply
	echo "Existing nonce mismatch: have $nonce, want 0x100000"
	exit 1
fi

# Query the nonce of a non-existent account
reply=`curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getTransactionCount","params":["0x1010101010101010101010101010101010101010", "latest"],"id":1}' "$HIVE_CLIENT_IP:8545"`

nonce=`echo $reply | sed 's/,/\n/g' | sed 's/}/\n/g' | grep result | cut -d ':' -f 2 | tr -d '"'`
if [ $nonce != "0x100000" ]; then
 echo "Existing nonce mismatch: have $nonce, want 0x100000"
 exit 1
fi
