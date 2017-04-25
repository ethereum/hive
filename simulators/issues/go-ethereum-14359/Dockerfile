# This is a regression test to cover a go-ethereum issue where the Ethereum net
# ID was represented with multiple different types (int, uint32 and uint64) in
# different code paths, causing nodes to be unable to connect to each other if
# the network ID was larger that maxuint32.
#
# Details in https://github.com/ethereum/go-ethereum/issues/14359
#
# The test starts up a network of two nodesand checks that they can successfully
# connect to each other.
FROM alpine:latest

# Build a bootnode to cross-discover the two plain nodes
RUN \
  apk add --update go git make gcc musl-dev linux-headers     && \
  git clone --depth 1 https://github.com/ethereum/go-ethereum && \
  (cd go-ethereum && make all)                                && \
  cp go-ethereum/build/bin/bootnode /bootnode                 && \
  apk del go git make gcc musl-dev                            && \
  rm -rf /go-ethereum && rm -rf /var/cache/apk/*

# Install curl to allow controlling the simulator as well as to interact with the
# client via HTTP-RPC.
RUN apk add --update bash curl jq

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json

# Add the simulation controller
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
