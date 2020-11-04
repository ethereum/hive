# This simulation runs discv4 protocol tests.
FROM golang:1-alpine as builder
RUN apk add --update git gcc musl-dev linux-headers

# Build devp2p tool.
RUN git clone --depth 1 https://github.com/ethereum/go-ethereum.git /go-ethereum
WORKDIR /go-ethereum
RUN go build -v ./cmd/devp2p

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v

# Build the simulation run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /go-ethereum/devp2p ./devp2p
COPY --from=builder /source/discv4 ./discv4
ENTRYPOINT ["./discv4"]
