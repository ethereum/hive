# This simulation runs Berachain RPC compatibility tests.
FROM golang:1-alpine as builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --update git ca-certificates gcc musl-dev linux-headers

# Build the simulator executable
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/berachain-rpc-compat .

# Copy tests into container
COPY tests ./tests

ENTRYPOINT ["./berachain-rpc-compat"]