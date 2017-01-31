# Docker container spec defining and running the simplest possible smoke test
# validating whether a client can import a genesis definition.
FROM alpine:latest

# Install curl to allow querying the HTTP-RPC endpoint
RUN apk add --update curl

# Inject the chain definition
ADD genesis.json /genesis.json

# Inject the tester and set the entrypoint
ADD test.sh /test.sh
RUN chmod +x /test.sh

ENTRYPOINT ["/test.sh"]
