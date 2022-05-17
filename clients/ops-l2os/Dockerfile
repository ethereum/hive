FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone https://github.com/ethereum-optimism/optimistic-specs /app

WORKDIR /app
RUN make op-proposer

FROM alpine:3.15

RUN apk add --no-cache curl jq

COPY --from=builder /app/op-proposer/bin/op-proposer /usr/local/bin
COPY l2os-entrypoint.sh /l2os-entrypoint.sh

RUN op-proposer --version 2>/dev/null | head -1 > /version.txt

# TODO
# RUN export L2OO_ADDRESS=

ENTRYPOINT ["/bin/sh", "/l2os-entrypoint.sh"]
