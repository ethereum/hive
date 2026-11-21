# This simulator runs devp2p protocol tests.

# Build devp2p tool.
FROM golang:1-alpine as geth-builder
ARG GOPROXY
ARG GETH_REPOSITORY=https://github.com/ethereum/go-ethereum.git
ARG GETH_REF=master
ENV GOPROXY=${GOPROXY}
RUN apk add --update git gcc musl-dev linux-headers
RUN git init /go-ethereum && \
    cd /go-ethereum && \
    git remote add origin "${GETH_REPOSITORY}" && \
    git fetch --depth 1 origin "${GETH_REF}" && \
    git checkout --detach FETCH_HEAD
WORKDIR /go-ethereum
RUN go build -v ./cmd/devp2p

# Build the simulator executable.
FROM golang:1-alpine as sim-builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
RUN apk add --update git gcc musl-dev linux-headers
WORKDIR /source
ADD go.mod go.sum ./
ADD simulators/devp2p/go.mod simulators/devp2p/go.sum ./simulators/devp2p/
WORKDIR /source/simulators/devp2p
RUN go mod download
WORKDIR /source
ADD hivesim ./hivesim
ADD internal ./internal
ADD simulators/devp2p ./simulators/devp2p
WORKDIR /source/simulators/devp2p
RUN go build -v -o devp2p-simulator

# Build the simulation run container.
FROM alpine:latest
ADD simulators/devp2p /source
WORKDIR /source
COPY --from=geth-builder /go-ethereum/devp2p ./devp2p
COPY --from=geth-builder /go-ethereum/cmd/devp2p/internal/ethtest/testdata /testchain
COPY --from=sim-builder /source/simulators/devp2p/devp2p-simulator .
ENTRYPOINT ["./devp2p-simulator"]
