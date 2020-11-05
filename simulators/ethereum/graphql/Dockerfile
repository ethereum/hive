# This simulation runs GraphQL tests.
FROM golang:1-alpine as builder
RUN apk add --update git gcc musl-dev linux-headers

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/graphql .
ENTRYPOINT ["./graphql"]
