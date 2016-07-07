# This simulation tests that miners pushing for the dao hard-fork set the correct
# extradata field for the fork block.
FROM alpine:3.3

# Install curl to allow controlling the simulator as well as to interact with the
# client via HTTP-RPC, bash and jq to allow writing a bit more exotic test scripts.
RUN apk add --update bash curl jq

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json
RUN touch chain.rlp && mkdir /blocks

ENV HIVE_FORK_DAO 3

# Add the simulation controller
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
