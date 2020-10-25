# This simulation runs various eth-protocol tests
FROM golang:1-alpine as builder
RUN apk add --update git make gcc musl-dev curl linux-headers

# Build devp2p tool.
RUN git clone --depth 1 https://github.com/ethereum/go-ethereum.git /go-ethereum
WORKDIR /go-ethereum
RUN go build -v -o ../devp2p ./cmd/devp2p

# Build the test.
ADD /ethereum/eth/ /eth
WORKDIR /eth
RUN go build -v

ENTRYPOINT ["./eth"]
