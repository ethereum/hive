# This is a regression test to cover an go-ethereum issue introduced after the
# full/light client API splits that broke CALL operations on develop.
#
# Details in https://github.com/ethereum/go-ethereum/issues/2796.
#
# The test deploys a greeter contract in block #1 and tried to call it's `Greet`
# method with the same account used to deploy the contract.
#
# contract HelloWorld {
#     function Greet() constant returns (string) {
#         return "Hello, World!";
#     }
# }
FROM alpine:latest

# Install curl to allow querying the HTTP-RPC endpoint
RUN apk add --update curl jq

# Inject the chain definition
ENV HIVE_FORK_HOMESTEAD 1150000

ADD genesis.json /genesis.json
ADD chain.rlp /chain.rlp

# Inject the tester and set the entrypoint
ADD test.sh /test.sh
RUN chmod +x /test.sh

ENTRYPOINT ["/test.sh"]
