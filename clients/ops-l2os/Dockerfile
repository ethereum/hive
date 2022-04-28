FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone https://github.com/ethereum-optimism/optimistic-specs /app

WORKDIR /app
RUN make l2os

FROM alpine:3.15

RUN apk add --no-cache curl jq

COPY --from=builder /app/bin/l2os /usr/local/bin
COPY l2os-entrypoint.sh /l2os-entrypoint.sh

RUN /app/bin/l2os --version 2>/dev/null | head -1 > /version.txt

# TODO
# RUN export L2OO_ADDRESS=

ENTRYPOINT ["/bin/sh", "/l2os-entrypoint.sh"]
