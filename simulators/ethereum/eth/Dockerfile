# This simulation runs various graphql tests
FROM golang:1.15-alpine as builder
RUN apk add --no-cache git make gcc musl-dev linux-headers bash  curl jq

RUN GOPATH=/go


ADD /ethereum/eth/init/ /init/
ADD /common /go/src/github.com/ethereum/hive/simulators/common
ADD /ethereum/eth/ /go/src/github.com/ethereum/hive/simulators/ethereum/eth/

RUN go build /go/src/github.com/ethereum/hive/simulators/ethereum/eth

RUN git clone https://github.com/ethereum/go-ethereum.git
RUN cd go-ethereum && go build ./cmd/devp2p -o ../devp2p

ENTRYPOINT ["./eth"]
