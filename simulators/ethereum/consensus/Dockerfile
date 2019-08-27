# Build Geth in a stock Go builder container
FROM golang:1.11-alpine as builder

RUN apk add --no-cache git make gcc musl-dev linux-headers bash
RUN GOPATH=/go 
RUN go get -u -v github.com/derekparker/delve/cmd/dlv
RUN go get -u -v github.com/google/uuid

# Get master branch of geth
RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)

# Add the local test stuff, preserving paths for remote debugger
ADD /ethereum/consensus/main.go /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/main.go
ADD /ethereum/consensus/simulator_test.go  /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/simulator_test.go
ADD /ethereum/consensus/types.go  /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/types.go
ADD common /go/src/github.com/ethereum/hive/simulators/common
RUN apk add --no-cache ca-certificates git

RUN git clone --depth 1 https://github.com/ethereum/tests.git /tests



ENV TESTPATH /tests

# Add the entry script
ADD /ethereum/consensus/tests.sh /tests.sh
RUN chmod +x /tests.sh

ENTRYPOINT ["/tests.sh"]
