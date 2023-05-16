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

# Build the simulator executable.
ENV GOPATH /Users/maceo/go
RUN go install github.com/go-delve/delve/cmd/dlv@latest
ADD . /Users/maceo/go/src/github.com/gnosischain/hive/simulators/ethereum/engine
WORKDIR /Users/maceo/go/src/github.com/gnosischain/hive/simulators/ethereum/engine
RUN go build -gcflags="all=-N -l" -v .

# Build the simulator run container.
FROM alpine:latest
#ADD . /Users/maceo/go/src/github.com/gnosischain/hive/simulators/ethereum/engine
WORKDIR /Users/maceo/go/src/github.com/gnosischain/hive/simulators/ethereum/engine
COPY --from=builder /Users/maceo/go/src/github.com/gnosischain/hive/simulators/ethereum/engine .
COPY --from=geth    /ethash /ethash
COPY --from=builder /Users/maceo/go/bin/dlv /go/bin/dlv

EXPOSE 40000

ENTRYPOINT ["/go/bin/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "--accept-multiclient", "exec", "./engine", "--", "serve"]
