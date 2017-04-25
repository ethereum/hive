# This is a regression test to cover a go-ethereum issue introduced after the EDGE
# release where sync was optimised to discard inbound transactions until an initial
# sync cycle completes. However, for single miner private network scenarios the
# miner itself never completes this cycle, so it never accepts transactions.
#
# Details in https://github.com/ethereum/go-ethereum/issues/2769
#
# The test starts up a network of two nodes (a miner and a non miner) and checks
# that transactions from both nodes propagate to the other.
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
