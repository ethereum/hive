# This is a regression test to cover an go-ethereum issue of importing a testnet
# chain from an exported RLP dump. It ensures that testnet nonces are correctly
# handled for chain initialization as well as import operations.
#
# Details in https://github.com/ethereum/go-ethereum/issues/2905
FROM alpine:latest

# The test is trivial, just initializes a chain and returns
ADD genesis.json /genesis.json
ADD chain.rlp /chain.rlp
ENV HIVE_TESTNET 1

ENTRYPOINT ["echo", "noop"]
