# Generate the ethash verification caches.
# Use a static version because this will never need to be updated.
FROM ethereum/client-go:v1.10.20 AS geth
RUN \
 /usr/local/bin/geth makecache  1     /ethash && \
 /usr/local/bin/geth makecache  30000 /ethash && \
 /usr/local/bin/geth makedag    1     /ethash && \
 /usr/local/bin/geth makedag    30000 /ethash

# This simulation runs Engine API tests.
FROM golang:1-alpine as builder
RUN apk add --update gcc musl-dev linux-headers

# Set the GOPATH and enable Go modules
ENV GOPATH=/go
ENV GO111MODULE=on

# Build the simulator executable.
RUN go install github.com/go-delve/delve/cmd/dlv@latest
ADD . $GOPATH/src/github.com/gnosischain/hive/simulators/ethereum/engine
WORKDIR $GOPATH/src/github.com/gnosischain/hive/simulators/ethereum/engine
RUN go build  -gcflags="all=-N -l" -v .

# Build the simulator run container.
FROM alpine:latest
# Set the GOPATH and enable Go modules
ENV GOPATH=/go
ENV GO111MODULE=on
WORKDIR /app
COPY --from=builder $GOPATH/src/github.com/gnosischain/hive/simulators/ethereum/engine .
COPY --from=geth    /ethash /ethash
COPY --from=builder /go/bin/dlv /go/bin/dlv

EXPOSE 40000

ENTRYPOINT ["/go/bin/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "./engine", "--", "serve"]
