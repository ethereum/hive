ARG branch=develop

FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /app

WORKDIR /app/op-node
RUN make op-node

FROM alpine:3.15

RUN apk add jq curl

COPY --from=builder /app/op-node/bin/op-node /usr/local/bin
COPY ./opnode-entrypoint.sh /opnode-entrypoint.sh
COPY ./rollup.json /rollup.json
RUN mkdir /hive /config
COPY p2p-node-key.txt /config
COPY p2p-sequencer-key.txt /config
COPY test-jwt-secret.txt /config

RUN op-node --version 2>/dev/null | head -1 > /version.txt

EXPOSE 7545/tcp

ENTRYPOINT ["/bin/sh", "/opnode-entrypoint.sh"]
