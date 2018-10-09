# This validation runs various devp2p tests against the client

FROM golang:1-alpine

# Build a geth bootnode, regular node from the develop branch and unit test binary
RUN apk add --update git make gcc musl-dev curl jq linux-headers

# Get master branch of geth
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)
RUN (cd /go/src/github.com/ethereum/go-ethereum )

# Build geth
RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go make all)
RUN (cd /go/src/github.com/ethereum/go-ethereum && GOPATH=/go go get)

# Add the local test stuff
ADD devp2p_test.go /devp2p_test.go
ADD node.go /node.go
ADD udp.go /udp.go

#go test -c -o means compile the tests (-c) and rename (-o) to 
RUN (cd / && GOPATH=/go go test -c -o /devp2p.test)

# Cleanup previous stuff 
RUN apk del git make gcc musl-dev linux-headers
RUN (rm -rf /var/cache/apk/* && rm -rf /go)

# Add shell
RUN apk add --update bash
# Add the entry script
ADD tests.sh /tests.sh
#mandatory init file
ADD genesis.json /genesis.json

ENTRYPOINT ["/tests.sh"]
