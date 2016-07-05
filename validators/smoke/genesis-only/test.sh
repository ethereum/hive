#!/bin/sh
set -e

# Simply query the genesis block and check that it's correct
reply=`curl -s -X POST --data '{"jsonrpc":"2.0","method":"eth_getBlockByNumber","params":["0x0", false],"id":0}' "$CLIENT_PORT_8545_TCP_ADDR:8545"`

hash=`echo $reply | sed 's/,/\n/g' | grep hash | cut -d ':' -f 2 | tr -d '"'`
test "$hash" == "0xbd008bffd224489523896ed37442e90b4a7a3218127dafdfed9d503d95e3e1f3"
