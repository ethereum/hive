# Generate the ethash verification caches.
# Use a static version because this will never need to be updated.
FROM ethereum/client-go:v1.10.20 AS geth
RUN \
 /usr/local/bin/geth makecache  1 /ethash && \
 /usr/local/bin/geth makedag    1 /ethash

# Build the simulator binary
FROM golang:1-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers cmake make clang build-base clang-static clang-dev

# Prepare workspace
ADD ./engine/hive.go.work /source/go.work
ADD ./common /source/common
ADD ./engine /source/engine
ADD ./engine/hive.go.work /source/go.work

# Build within simulator folder
WORKDIR /source/engine
RUN go build -o ./sim .

# Build the runner container.
FROM alpine:latest
ADD . /
COPY --from=builder /source/engine/sim /
COPY --from=geth    /ethash /ethash

ENTRYPOINT ["./sim"]
