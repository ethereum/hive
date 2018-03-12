# This benchmark imports a pre-generated chain containing the ZkSnarks contract.
#
# For details see https://gist.github.com/chriseth/f9be9d9391efc5beb9704255a8e2989d#file-snarktest-solidity
FROM mhart/alpine-node

# Install web3 to allow interacting with the tracer
RUN apk add --no-cache git
RUN npm install web3@0.19.0

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json
ADD chain.rlp /chain.rlp

ENV HIVE_FORK_HOMESTEAD  0
ENV HIVE_FORK_TANGERINE  0
ENV HIVE_FORK_SPURIOUS   0
ENV HIVE_FORK_METROPOLIS 0

# Add the benchmark controller
ADD benchmark.js /benchmark.js
ENTRYPOINT ["node", "/benchmark.js"]
