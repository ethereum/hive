# Build Geth in a stock Go builder container
FROM golang:1.18-alpine as builder

RUN apk add --no-cache gcc musl-dev linux-headers git

RUN git clone https://github.com/ethereum-optimism/reference-optimistic-geth /go-ethereum

RUN cd /go-ethereum && go run build/ci.go install ./cmd/geth

# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache jq ca-certificates
COPY --from=builder /go-ethereum/build/bin/geth /usr/local/bin/

RUN /usr/local/bin/geth console --exec 'console.log(admin.nodeInfo.name)' --maxpeers=0 --nodiscover --dev 2>/dev/null | head -1 > /version.txt

# Regenerate the L2 genesis file
RUN mkdir /hive
ADD genesis.json /genesis.json

# Inject the startup script
ADD entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

# Helper scripts; called from devnet.go
ADD cat.sh /hive-bin/cat.sh
RUN chmod +x /hive-bin/cat.sh

VOLUME ["/db"]

EXPOSE 9545/tcp
EXPOSE 9546/tcp

ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
