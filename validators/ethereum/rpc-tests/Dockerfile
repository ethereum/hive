# Docker container spec defining and running the official RPC JSON test suite.
FROM ubuntu:16.04

# Install all the node.js dependencies for the RPC test suite
RUN \
  apt-get update                                                                 && \
  apt-get install -y git ca-certificates nodejs npm make --no-install-recommends && \
  ln -s /usr/bin/nodejs /usr/bin/node

# Download and install the test repository itself
RUN \
  git clone https://github.com/ethereum/rpc-tests && \
	cd rpc-tests                                    && \
	git submodule update --init                     && \
	npm install

# Hack the tester to connect to a remote node, not local one
RUN sed -i -- "s#'http://localhost:8545'#'http://' + process.env.CLIENT_PORT_8545_TCP_ADDR + ':8545'#g" /rpc-tests/lib/config.js

# Configure the blockchain for testing
ADD genesis.json /genesis.json
ADD makechain.js /makechain.js
RUN nodejs makechain.js

WORKDIR /rpc-tests
ENTRYPOINT ["make", "test"]
