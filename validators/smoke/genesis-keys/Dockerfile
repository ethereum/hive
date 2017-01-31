# Docker container spec defining and running a simple smoke test to ensure that
# a node can initialize itself and load a batch of account keys defined in the
# web3 secret storage definition format.
#
# Details at https://github.com/ethereum/wiki/wiki/Web3-Secret-Storage-Definition
FROM alpine:latest

# Install curl to allow querying the HTTP-RPC endpoint
RUN apk add --update curl

# Inject the chain definition and the initial keys
ADD genesis.json /genesis.json
ADD keys.tar.xz /keys

# Inject the tester and set the entrypoint
ADD test.sh /test.sh
RUN chmod +x /test.sh

ENTRYPOINT ["/test.sh"]
