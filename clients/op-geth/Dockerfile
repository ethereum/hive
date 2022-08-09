# Build Geth in a stock Go builder container
FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

ARG branch=optimism

RUN git clone -b $branch https://github.com/ethereum-optimism/reference-optimistic-geth /go-ethereum

RUN cd /go-ethereum && go run build/ci.go install ./cmd/geth

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add curl jq bash
RUN apk add --no-cache ca-certificates

COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/

RUN geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/bin/bash", "/entrypoint.sh"]
