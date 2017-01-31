# Docker container spec defining and running a simple smoke test consisting of
# starting a single client node and retrieving some data from it.
FROM alpine:latest

# Install curl to allow controlling the simulator as well as to interact with the
# client via HTTP-RPC.
RUN apk add --update curl

# Configure the initial setup for the individual nodes
ADD genesis.json /genesis.json

# Add the simulation controller
ADD simulator.sh /simulator.sh
RUN chmod +x /simulator.sh

ENTRYPOINT ["/simulator.sh"]
