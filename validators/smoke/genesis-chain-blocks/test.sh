#!/bin/sh
set -e

# Simply query the 8th block and check that it's correct
reply=`curl -s -X POST -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x8", false],"id":1}' "$HIVE_CLIENT_IP:8545"`

hash=`echo $reply | sed 's/,/\n/g' | grep hash | cut -d ':' -f 2 | tr -d '"'`

test $hash == "0xb644747889b47b8e61c8a0a452445168df5ab1cf7f83454c2983a2f45827a8b1"
