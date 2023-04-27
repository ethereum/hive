## By default, geth is pulled from Docker Hub.

ARG user=ethereum
ARG repo=client-go
ARG branch=latest
FROM $user/$repo:$branch as builder

## ----
## Uncomment the steps below (and comment out the steps above!) to build
## go-ethereum from a git branch.

# FROM alpine:latest as builder
# ARG user=ethereum
# ARG repo=go-ethereum
# ARG branch=master
# RUN \
#   apk add --update bash curl jq go git make gcc musl-dev         \
#         ca-certificates linux-headers                         && \
#   git clone --depth 1 --branch $branch                           \
#       https://github.com/$user/$repo                          && \
#   (cd go-ethereum && make geth)                               && \
#   (cd go-ethereum                                             && \
#   echo "{}"                                                      \
#   | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}" \
#   | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"  \
#   | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"               \
#   > /version.json)                                            && \
#   cp go-ethereum/build/bin/geth /usr/local/bin/geth           && \
#   apk del go git make gcc musl-dev linux-headers              && \
#   rm -rf /go-ethereum && rm -rf /var/cache/apk/*

## ----

FROM alpine:latest
RUN apk add --update bash curl jq
COPY --from=builder /usr/local/bin/geth /usr/local/bin/geth

# Generate the version.txt file.
RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

# Inject the startup script.
ADD geth.sh /geth.sh
ADD mapper.jq /mapper.jq
RUN chmod +x /geth.sh

# Inject the enode id retriever script.
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Add a default genesis file.
ADD genesis.json /genesis.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8547 8551 30303 30303/udp

ENTRYPOINT ["/geth.sh"]
