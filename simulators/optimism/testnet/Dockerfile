# This simulation runs JSON-RPC API tests.
FROM golang:1.18-alpine3.15 as builder
RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable, from hive repo root.
# See context.txt for docker build context change.
# We use a go.work file to pull in other go modules of the hive repo.
ADD ./simulators/optimism/testnet/hive.go.work /source/go.work
ADD ./optimism /source/optimism
ADD ./simulators/optimism/testnet /source/testnet

WORKDIR /source/testnet
RUN go build -v .


# Build the simulator run container.
FROM alpine:latest

COPY --from=builder /source/testnet/testnet .
ENTRYPOINT ["./testnet"]
