# Build Geth in a stock Go builder container
FROM golang:1.11-alpine as builder

RUN apk add --no-cache git make gcc musl-dev linux-headers

RUN go get github.com/holiman/goconsensus && go install github.com/holiman/goconsensus


# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates git
COPY --from=builder /go/bin/goconsensus /usr/local/bin/goconsensus
RUN git clone --depth 1 https://github.com/ethereum/tests.git /tests

RUN chmod a+x /usr/local/bin/goconsensus

ENV TESTPATH /tests

CMD ["/usr/local/bin/goconsensus"]
