FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

ARG branch=develop

RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /app

WORKDIR /app/op-node
RUN make op-node

FROM alpine:3.15

RUN apk add curl jq bash
RUN apk add --no-cache ca-certificates

COPY --from=builder /app/op-node/bin/op-node /usr/local/bin

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

RUN op-node --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
