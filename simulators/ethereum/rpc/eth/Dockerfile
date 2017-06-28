# This simulation tests the RPC interface on a "live" chain using
# the go RPC client that is part of the go-ethereum repo.
#
# The simulation starts a bootnode with multipe client nodes that find each
# other through the bootnode.
FROM golang:1-alpine

# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl jq linux-headers

ENV GOPATH /gopath

# Checkout source, pin version to ensure that RPC test suite doesn't break due
# to incompatibility issues in the client packages/generated abi sources.
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum $GOPATH/src/github.com/ethereum/go-ethereum)
RUN (cd $GOPATH/src/github.com/ethereum/go-ethereum && git checkout 9e5f03b6c487175cc5aa1224e5e12fd573f483a7)
RUN (go get golang.org/x/net/context)

# Build and copy binaries to /
RUN (cd $GOPATH/src/github.com/ethereum/go-ethereum && make all)
RUN cp $GOPATH/src/github.com/ethereum/go-ethereum/build/bin/bootnode /bootnode
RUN cp $GOPATH/src/github.com/ethereum/go-ethereum/build/bin/abigen /abigen

# copy test source files
ADD eth_test.go /eth_test.go
ADD ethclient.go /ethclient.go
ADD abi.go /abi.go
ADD helper.go /helper.go
ADD vault.go /vault.go

# Generate ABI bindings
ADD contractABI.json /contractABI.json
RUN (cd / && /abigen -abi /contractABI.json -pkg main -type TestContract -out /ABI_generated.go)

# build test binary (/rpc.test is run from simulator.sh)
RUN (cd / && go test -c -o /rpc.test)

# Cleanup dependencies
RUN apk del git make gcc musl-dev linux-headers
RUN (rm -rf /var/cache/apk/* && rm -rf $GOPATH)

RUN apk add --update bash

# Each node has its own key with 123.45 preallocated ether in the genesis block
RUN mkdir /keystores1 /keystores2 /keystores3 /keystores4
RUN mkdir /keystores5 /keystores6 /keystores7 /keystores8
ADD keys/cf49fda3be353c69b41ed96333cd24302da4556f /keystores1/
ADD keys/b97de4b8c857e4f6bc354f226dc3249aaee49209 /keystores2/
ADD keys/87da6a8c6e9eff15d703fc2773e32f6af8dbe301 /keystores3/
ADD keys/0161e041aad467a890839d5b08b138c1e6373072 /keystores4/
ADD keys/c5065c9eeebe6df2c2284d046bfc906501846c51 /keystores5/
ADD keys/c0d7b9d445f0302eadcc1b7b69acf8ad4f89d7e4 /keystores6/
ADD keys/4b40d7c6d5023bceb42510684d3348abd24f5e0f /keystores7/
ADD keys/43de7389c3c874955f33538e746f3f9ed8417d6f /keystores8/

# Node bootstrap script expects this file/dir to exist
RUN touch /chain.rlp && mkdir /blocks

# Inject the simulator startup script and related resources
ADD genesis.json /genesis.json
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
