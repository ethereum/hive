# This simulation tests that if there are node both for and against the DAO hard
# fork in a single network, then the moment the hard fork commences the two sides
# will partition apart and never merge back.
#
# This is important because at a protocol/networking level the two blockchains
# are still fully compatible with each other, only content wise do they differ.
# However if they do not split apart, then all kinds of anomalies will arise when
# nodes from different forks try to sync with each other, especially during the
# fast and light sync algos which do not have access to historical content to
# verify which chain their peer's on.
FROM alpine:3.3

# The test needs a fork-independent bootstrap node. Build from the go-ethereum repo.
RUN \
  apk add --update go git make gcc musl-dev         && \
  git clone https://github.com/ethereum/go-ethereum && \
  (cd go-ethereum && git checkout develop)          && \
  (cd go-ethereum && make all)                      && \
  cp go-ethereum/build/bin/bootnode /bootnode       && \
  apk del go git make gcc musl-dev                  && \
  rm -rf /go-ethereum && rm -rf /var/cache/apk/*

RUN apk add --update bash curl jq

# Configure the clients for the simulation
ADD genesis.json /genesis.json
RUN touch chain.rlp && mkdir /blocks
ENV HIVE_TESTNET 1

# Inject the simulator startup script
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

# Export the devp2p data port to allow running a local bootstrap node
EXPOSE 30303

ENTRYPOINT ["/simulator.sh"]
