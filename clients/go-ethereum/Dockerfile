## By default, the geth is pulled from Docker Hub.

ARG branch=latest
FROM ethereum/client-go:$branch as builder

## ----
## Uncomment the steps below (and comment out the steps above!) to build go-ethereum
## from local sources in the ./go-ethereum directory.

# FROM golang:1-alpine as builder
# ARG branch=master
# ADD go-ethereum /go-ethereum
# WORKDIR /go-ethereum
# RUN apk add --update bash curl jq git make
# RUN make geth
# RUN mv ./build/bin/geth /usr/local/bin/geth

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
