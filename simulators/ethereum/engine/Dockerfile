# This simulation runs Engine API tests.
FROM golang:1-alpine as builder
RUN apk add --update gcc musl-dev linux-headers git

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go mod download github.com/ethereum/go-ethereum && go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/engine .
ENTRYPOINT ["./engine"]
