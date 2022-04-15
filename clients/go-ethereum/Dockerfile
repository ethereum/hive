# Docker container spec for building the master branch of go-ethereum.
#
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.

ARG branch=latest
FROM ethereum/client-go:$branch

RUN apk add --update bash curl jq

# FROM alpine:latest
# ARG branch=master

# Build go-ethereum on the fly and delete all build tools afterwards
# RUN \
#   apk add --update bash curl jq go git make gcc musl-dev              \
#         ca-certificates linux-headers                           && \
#   git clone --depth 1 --branch $branch  https://github.com/ethereum/go-ethereum && \
#   (cd go-ethereum && make geth)                               && \
#   (cd go-ethereum                                             && \
#   echo "{}"                                                      \
#   | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
#   | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
#   | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
#   > /version.json)                                            && \
#   cp go-ethereum/build/bin/geth /geth                         && \
#   apk del go git make gcc musl-dev linux-headers              && \
#   rm -rf /go-ethereum && rm -rf /var/cache/apk/*

RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

# Inject the startup script
ADD geth.sh /geth.sh
ADD mapper.jq /mapper.jq
RUN chmod +x /geth.sh

# Inject the enode id retriever script
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

ADD genesis.json /genesis.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8547 8551 30303 30303/udp

# Generate the ethash verification caches
RUN \
 /usr/local/bin/geth makecache     1 ~/.ethereum/geth/ethash && \
 /usr/local/bin/geth makecache 30001 ~/.ethereum/geth/ethash

ENTRYPOINT ["/geth.sh"]
