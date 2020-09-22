#!/bin/sh

set -e

if test -f enode.json; then
	rm enode.json
fi

if test -f geth.log; then
	rm geth.log
fi


# start configured geth node
sudo add-apt-repository -y ppa:ethereum/ethereum
sudo apt-get update
sudo apt-get install ethereum

/usr/bin/geth --datadir=testchain-data/ --verbosity=5 --nat=none --nodiscover --nousb --http --http.port 7474 --http.api admin > geth.log 2>&1 &

# run the test suite against the node
sleep 2
curl -H "Content-type:application/json" -XPOST -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":67}' http://127.0.0.1:7474 >> enode.json

# parse enode
go build parseJson.go
ENODE=$(go run parseJson.go)


./devp2p rlpx eth-test ${ENODE} chain.rlp.gz genesis.json

trap 'pkill geth' EXIT

