# Build the simulator.
FROM golang:1-alpine AS builder
RUN apk --no-cache add gcc musl-dev linux-headers
ADD . /clique
WORKDIR /clique
RUN go build .

# Build the runner container.
FROM alpine:latest
ADD . /
COPY --from=builder /clique/clique /
ENTRYPOINT ["./clique"]
