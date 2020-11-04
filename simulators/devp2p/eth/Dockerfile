# This simulation runs eth protocol tests.
FROM golang:1-alpine as builder
RUN apk add --update git make gcc musl-dev curl linux-headers

# Build devp2p tool.
RUN git clone --depth 1 https://github.com/ethereum/go-ethereum.git /go-ethereum
WORKDIR /go-ethereum
RUN go build -v ./cmd/devp2p

# Build the simulator executable.
ADD . /eth
WORKDIR /eth
RUN go build -v

# Build the simulation run container.
FROM alpine:latest
ADD . /eth/
WORKDIR /eth
COPY --from=builder /go-ethereum/devp2p /devp2p
COPY --from=builder /eth/eth /eth/eth
ENTRYPOINT ["/eth/eth"]
