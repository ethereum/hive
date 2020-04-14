# To build this in itself, it needs to be built from a parent
# folder, in order to obtain common tools: 
#
# [~/go/src/github.com/ethereum/hive/simulators]
# docker build -f ./ethereum/consensus/Dockerfile  .

# Build Geth in a stock Go builder container
FROM golang:1.14-alpine as builder

RUN apk add --no-cache git make gcc musl-dev linux-headers bash
RUN GOPATH=/go 

ENV GO111MODULE=on

#ENV GO111MODULE=off
#RUN go get -u -v github.com/go-delve/delve/cmd/dlv
#ENV GO111MODULE=on


# Add the local test stuff, preserving paths for remote debugger
ADD /ethereum/consensus/main.go /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/main.go
ADD /ethereum/consensus/simulator_test.go  /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/simulator_test.go
ADD /ethereum/consensus/types.go  /go/src/github.com/ethereum/hive/simulators/ethereum/consensus/types.go
ADD common /go/src/github.com/ethereum/hive/simulators/common
RUN apk add --no-cache ca-certificates git

RUN git clone --depth 1 https://github.com/ethereum/tests.git /tests

ENV TESTPATH /tests

# Use modules (temp fix until sims are sub modules)
RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)

# Add the entry script
ADD /ethereum/consensus/tests.sh /tests.sh
RUN chmod +x /tests.sh

ENTRYPOINT ["/tests.sh"]
