# This simulation runs JSON-RPC API tests.
FROM golang:1.18-alpine3.15 as builder
RUN apk add --update gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/p2p .
ENTRYPOINT ["./p2p"]
