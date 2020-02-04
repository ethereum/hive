# This simulation runs various graphql tests
FROM golang:1.13-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq
RUN GOPATH=/go 

ENV GO111MODULE=off
RUN go get -u -v github.com/derekparker/delve/cmd/dlv
ENV GO111MODULE=on



# Add the local test files
ADD /ethereum/graphql/graphql.go /go/src/github.com/ethereum/hive/simulators/ethereum/graphql/graphql.go
ADD common /go/src/github.com/ethereum/hive/simulators/common

# Add the tests and intended test chain
ADD /ethereum/graphql/testcases/error /testcases/error
ADD /ethereum/graphql/testcases/eth /testcases/eth
ADD /ethereum/graphql/init /init


# Use modules (temp fix until sims are sub modules)
RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)


# Add the entry script
ADD /ethereum/graphql/tests.sh /tests.sh
RUN chmod +x /tests.sh

EXPOSE 2345 8545 8546 8080 30303 30303/udp 

ENTRYPOINT ["/tests.sh"]

