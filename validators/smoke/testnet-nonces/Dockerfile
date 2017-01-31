# Docker container spec verifying whether a client can configure itself to run a
# testnet, in particular that the starting nonces are indeed 2^20.
FROM alpine:latest

# Install curl to allow querying the HTTP-RPC endpoint
RUN apk add --update curl

# Inject the chain definition
ADD genesis.json /genesis.json

# Fine tune the client parameters
ENV HIVE_TESTNET 1

# Inject the tester and set the entrypoint
ADD test.sh /test.sh
RUN chmod +x /test.sh

ENTRYPOINT ["/test.sh"]
