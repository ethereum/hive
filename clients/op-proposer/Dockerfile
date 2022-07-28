FROM golang:1.18.0-alpine3.15 as builder

RUN apk add --no-cache make gcc musl-dev linux-headers git jq bash

ARG branch=develop

RUN git clone -b $branch https://github.com/ethereum-optimism/optimism /app

WORKDIR /app/op-proposer
RUN make

FROM alpine:3.15

RUN apk add curl jq bash
RUN apk add --no-cache ca-certificates

COPY ./entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

COPY --from=builder /app/op-proposer/bin/op-proposer /usr/local/bin

RUN op-proposer --version 2>/dev/null | head -1 > /version.txt

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
