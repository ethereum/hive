# This simulation runs JSON-RPC API tests.
FROM golang:1-alpine as builder
RUN apk add --update git ca-certificates gcc musl-dev linux-headers

ARG branch=devp2p-eth-cmd

# Build go-ethereum.
RUN \
  apk add --update bash curl jq go git make gcc musl-dev ca-certificates linux-headers && \
  git clone --branch $branch --depth 1 https://github.com/lightclient/go-ethereum.git /go-ethereum && \
  (cd /go-ethereum && make all)

# Clone the tests repo.
RUN git clone --branch add-debug --depth 1 https://github.com/lightclient/execution-apis.git /execution-apis

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/rpc-compat .
COPY --from=builder /execution-apis/tests ./tests
COPY --from=builder /go-ethereum/build/bin/devp2p .

ENTRYPOINT ["./rpc-compat"]
