FROM golang:1.13-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq
RUN GOPATH=/go

# Add the local test files
ADD /ethereum/iptest/iptest.go /go/src/github.com/ethereum/hive/simulators/ethereum/iptest/iptest.go
ADD common /go/src/github.com/ethereum/hive/simulators/common

RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)

RUN cd /go/src/github.com/ethereum/hive/simulators && go build -v ./ethereum/iptest
RUN cp /go/src/github.com/ethereum/hive/simulators/iptest .
RUN cd

ENTRYPOINT ["./iptest"]