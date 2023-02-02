# Generate the ethash verification caches.
# Use a static version because this will never need to be updated.
#FROM ethereum/client-go:v1.10.20 AS geth
FROM inphi/geth:eip4844-39ef7d7 as geth
RUN \
    /usr/local/bin/geth makecache  1     /ethash && \
    /usr/local/bin/geth makecache  30000 /ethash && \
    /usr/local/bin/geth makedag    1     /ethash && \
    /usr/local/bin/geth makedag    30000 /ethash

# This simulation runs Engine API tests.
FROM golang:1-alpine as builder
RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/engine .
COPY --from=geth    /ethash /ethash
ENTRYPOINT ["./engine"]
