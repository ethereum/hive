FROM golang:1.13-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq
RUN GOPATH=/go

# Add the local test files
ADD /smoke/network/network.go /go/src/github.com/ethereum/hive/simulators/smoke/network/network.go
ADD common /go/src/github.com/ethereum/hive/simulators/common

RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)

RUN cd /go/src/github.com/ethereum/hive/simulators && go build -v ./smoke/network
RUN cp /go/src/github.com/ethereum/hive/simulators/network .
RUN cd

ENTRYPOINT ["./network"]