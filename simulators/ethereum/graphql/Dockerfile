# This simulation runs GraphQL tests.
FROM golang:1-alpine as builder
RUN apk add --update git gcc musl-dev linux-headers

# Clone the tests repo.
RUN git clone --depth 1 https://github.com/ethereum/execution-apis.git /execution-apis

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/graphql .
COPY --from=builder /execution-apis/graphql/tests ./tests
ENTRYPOINT ["./graphql"]
