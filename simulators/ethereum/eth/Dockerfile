# This simulation runs various graphql tests
FROM golang:1.15-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq

RUN GOPATH=/go


ADD /ethereum/eth/init/ /init/
ADD /common /go/src/github.com/ethereum/hive/simulators/common
ADD /ethereum/eth/ /go/src/github.com/ethereum/hive/simulators/ethereum/eth/

RUN (cd /go/src/github.com/ethereum/hive/simulators/ && go mod init)

RUN cd /go/src/github.com/ethereum/hive/simulators && go build ./ethereum/eth
RUN cp /go/src/github.com/ethereum/hive/simulators/eth .
RUN cd

RUN git clone https://github.com/ethereum/go-ethereum.git
RUN cd go-ethereum && go build -o ../devp2p ./cmd/devp2p

ENTRYPOINT ["./eth"]
