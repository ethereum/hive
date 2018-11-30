# Build Geth in a stock Go builder container
FROM golang:1.11-alpine as builder

RUN apk add --no-cache git make gcc musl-dev linux-headers

#RUN (git clone -b master --single-branch https://github.com/ethereum/go-ethereum /go/src/github.com/ethereum/go-ethereum)
#RUN (git clone -b go_consensus --single-branch https://github.com/holiman/hive /go/src/github.com/ethereum/hive)

#ADD /*.go /go/src/goconsensus/
RUN go get github.com/holiman/goconsensus && go install goconsensus


# Pull Geth into a second stage deploy alpine container
FROM alpine:latest

RUN apk add --no-cache ca-certificates git
COPY --from=builder /go/bin/goconsensus /usr/local/bin/goconsensus
#COPY --from=builder /tests /tests
RUN git clone --depth 1 https://github.com/ethereum/tests.git /tests

RUN chmod a+x /usr/local/bin/goconsensus

ENV TESTPATH /tests

CMD ["/usr/local/bin/goconsensus"]
