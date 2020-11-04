FROM golang:1-alpine as builder
RUN apk add --update git gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /sync
WORKDIR /sync
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /sync
WORKDIR /sync
COPY --from=builder /sync/sync2 ./sync
ENTRYPOINT ["./sync"]
