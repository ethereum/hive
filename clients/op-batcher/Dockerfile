ARG branch=develop

FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /app

WORKDIR /app/op-batcher
RUN make op-batcher

FROM alpine:3.15

RUN apk add jq curl

COPY --from=builder /app/op-batcher/bin/op-batcher /usr/local/bin
COPY ./entrypoint.sh /entrypoint.sh
COPY ./rollup.json /rollup.json

RUN op-batcher --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
