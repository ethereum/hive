FROM golang:1-alpine as builder
RUN apk add --update gcc musl-dev linux-headers

# Get dependencies first. This improves caching behavior since they only need
# to be re-downloaded when go.mod changes.
COPY go.mod /source/
COPY go.sum /source/
WORKDIR /source
RUN go mod download

# Now build the proxy executable.
ADD . /source
RUN go build -o hive-proxy ./tool

# Pull the executable into a fresh image.
FROM alpine:latest
COPY --from=builder /source/hive-proxy .
EXPOSE 8081/tcp
ENTRYPOINT ./hive-proxy --addr :8081
