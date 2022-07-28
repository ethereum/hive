FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

ARG branch=develop

RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /app

WORKDIR /app/op-batcher
RUN make op-batcher

FROM alpine:3.15

RUN apk add curl jq bash
RUN apk add --no-cache ca-certificates

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

COPY --from=builder /app/op-batcher/bin/op-batcher /usr/local/bin

RUN op-batcher --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
