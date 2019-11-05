# This simulation runs various graphql tests
FROM golang:1.13-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash
RUN GOPATH=/go 
RUN go get -u -v github.com/derekparker/delve/cmd/dlv
RUN go get -u -v github.com/google/uuid
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)

# Add the local test files
ADD /ethereum/graphql/graphql.go /go/src/github.com/ethereum/hive/simulators/ethereum/graphql/graphql.go
ADD common /go/src/github.com/ethereum/hive/simulators/common

# Add the tests and intended test chain
ADD /ethereum/graphql/testcases/error /testcases/error
ADD /ethereum/graphql/testcases/eth /testcases/eth
ADD /ethereum/graphql/init /init



# Add shell
RUN apk add --update bash curl jq
# Add the entry script
ADD /ethereum/graphql/tests.sh /tests.sh
RUN chmod +x /tests.sh

EXPOSE 2345 8545 8546 8080 30303 30303/udp 

ENTRYPOINT ["/tests.sh"]

