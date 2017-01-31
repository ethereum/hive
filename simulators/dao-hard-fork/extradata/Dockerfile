# This simulation tests that miners pushing for the dao hard-fork set the correct
# extradata field for the fork block and the 10 consecutive blocks.
#
# This is important to allow header-only mode clients (fast sync algo, light
# client) to verify whether a header is indeed on the corrent chain and not get
# duped into syncing a live alternative fork. It also helps full nodes to split
# the network between two forks and avoid cross network noise.
#
# Consecutive blocks are needed to prevent any malicious miner from minting a
# non-forked block with the fork extradata. Unless almost all non-forking miners
# are malicious, there will be a non-forking block in the first 10 that does not
# have the fork extra-data set, separating that chain from the forked chain.
FROM alpine:latest

# Install curl to allow controlling the simulator as well as to interact with the
# client via HTTP-RPC, bash and jq to allow writing a bit more exotic test scripts.
RUN apk add --update bash curl jq

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json
ENV HIVE_FORK_DAO_BLOCK 3

# Add the simulation controller
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
