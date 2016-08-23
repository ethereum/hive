# Docker container spec defining and running a simple smoke test consisting of
# starting a single client node and measuring the genesis block retrival time.
FROM alpine:3.4

# Install curl to allow querying the HTTP-RPC endpoint
RUN apk add --update curl

# Configure the node
ADD genesis.json /genesis.json

# Inject the benchmarker and set the entrypoint
ADD benchmark.sh /benchmark.sh
RUN chmod +x /benchmark.sh

ENTRYPOINT ["/benchmark.sh"]
