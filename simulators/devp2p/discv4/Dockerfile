# This simulation runs various discovery v4 tests
FROM golang:1.15-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq

RUN GOPATH=/go

ADD /common /go/src/github.com/ethereum/hive/simulators/common
ADD /devp2p/discv4/ /go/src/github.com/ethereum/hive/simulators/devp2p/discv4/

RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)

RUN cd /go/src/github.com/ethereum/hive/simulators && go build ./devp2p/discv4/
RUN cp /go/src/github.com/ethereum/hive/simulators/discv4 .
RUN cd

RUN git clone --depth 1 https://github.com/ethereum/go-ethereum.git
RUN cd go-ethereum && go build -v -o ../devp2p ./cmd/devp2p

ENTRYPOINT ["./discv4"]
