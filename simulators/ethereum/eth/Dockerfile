FROM golang:1-alpine

RUN apk add --update git make gcc musl-dev curl jq linux-headers bash
ENV GO111MODULE=off

ADD ./devp2p  /go/src/github.com/ethereum/hive/simulators/ethereum/eth/devp2p
ADD ./chain.rlp.gz  /go/src/github.com/ethereum/hive/simulators/ethereum/eth/chain.rlp.gz
ADD ./genesis.json  /go/src/github.com/ethereum/hive/simulators/ethereum/eth/genesis.json
ADD ./parseJson.go /go/src/github.com/ethereum/hive/simulators/ethereum/eth/parseJson.go
ADD ./ethtest.sh  /go/src/github.com/ethereum/hive/simulators/ethereum/eth/ethtest.sh

RUN chmod +x /go/src/github.com/ethereum/hive/simulators/ethereum/eth/ethtest.sh

ENTRYPOINT ["/go/src/github.com/ethereum/hive/simulators/ethereum/eth/ethtest.sh"]