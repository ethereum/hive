# This simulation tests the shh RPC interface on a "live" chain using
# the go RPC client that is part of the go-ethereum repo.
#
# The simulation starts a bootnode with multipe whisper client nodes that
# find each other through the bootnode.
FROM golang:1-alpine

# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl jq linux-headers

ENV GOPATH /go

# Checkout source, pin version to ensure that RPC test suite doesn't break due
# to incompatibility issues introduced in new commits.
RUN (rm -rf /go/src/)
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)
RUN (cd /go/src/github.com/ethereum/go-ethereum && git checkout 5421a08d2f924558056c8a382017ec37f56010e9)

# Build binaries
RUN (cd /go/src/github.com/ethereum/go-ethereum && make all)

# Build and copy binaries to /
RUN (cd /go/src/github.com/ethereum/go-ethereum && make all)
RUN cp /go/src/github.com/ethereum/go-ethereum/build/bin/bootnode /bootnode

# Build test binary
ADD /shh_test.go /shh_test.go
ADD /helper.go /helper.go

RUN (cd / && go test -c -o /shh.test)

# Cleanup dependencies
RUN apk del git make gcc musl-dev linux-headers
RUN (rm -rf /var/cache/apk/* && rm -rf /go)

RUN apk add --update bash

# Inject the simulator startup script and related resources
ADD genesis.json /genesis.json

# Node bootstrap script expects this file/dir to exist
RUN touch /chain.rlp && mkdir /blocks

ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
