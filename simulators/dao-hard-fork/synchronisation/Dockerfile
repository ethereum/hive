# This simulation tests that freshly booted nodes within a network consisting of
# both pro- as well as no-forker nodes can correctly sync to their own chain of
# choice, irrelevant which that might be or whether slow or fast sync is being
# done.
FROM alpine:latest

# Build a bootnode and Geth to act as the two pro-fork and no-fork sources.
RUN \
  apk add --update go git make gcc musl-dev                   && \
  git clone --depth 1 https://github.com/ethereum/go-ethereum && \
  (cd go-ethereum && make all)                                && \
  cp go-ethereum/build/bin/bootnode /bootnode                 && \
  cp go-ethereum/build/bin/geth /geth                         && \
  apk del go git make gcc musl-dev                            && \
  rm -rf /go-ethereum && rm -rf /var/cache/apk/*

RUN apk add --update bash curl jq

# Configure the clients for the simulation
ADD genesis.json /genesis.json
ENV HIVE_FORK_DAO_BLOCK 128

# Inject the simulator startup script and related resources
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ADD nofork-chain.rlp.tar.xz /
ADD profork-chain.rlp.tar.xz /

# Export the devp2p data port to allow running a local bootstrap node
EXPOSE 30303 30304 30305

ENTRYPOINT ["/simulator.sh"]
