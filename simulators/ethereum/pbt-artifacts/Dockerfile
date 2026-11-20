FROM golang:1-alpine as builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}

RUN apk add --update git gcc musl-dev linux-headers

ADD . /pbt-artifacts
WORKDIR /pbt-artifacts
RUN go build -v .

# geth at geth_commit writes the valid pair; fixtures/gen, compiled inside
# that checkout, holds it to the pinned digests (EIPs#12404's tagged
# records) before cutting the cases. geth_commit is pbt at the merge of
# CPerezz/go-ethereum#47. Pinned to golang:1.25,
# not the floating 1-alpine tag: geth's cockroachdb/swiss dependency
# excludes Go 1.27.
FROM golang:1.25-alpine as fixtures
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
ARG geth=CPerezz/go-ethereum
ARG geth_commit=fbfd486b126fcdfe131dd94ac467ace881e0b31b

RUN apk add --update git gcc musl-dev linux-headers
# HTTP/1.1: Alpine's git 2.54 cannot fetch from GitHub over HTTP/2 (401).
RUN git init -q /go-ethereum && \
    git -C /go-ethereum -c http.version=HTTP/1.1 fetch -q --depth 1 https://github.com/${geth} ${geth_commit} && \
    git -C /go-ethereum checkout -q FETCH_HEAD
WORKDIR /go-ethereum
RUN go build -o /usr/local/bin/geth ./cmd/geth
COPY fixtures/gen/*.go fixtures/gen/pbt_root.py ./cmd/pbt-fixtures/
RUN mkdir /fixtures && go run -tags pbtgen ./cmd/pbt-fixtures -geth /usr/local/bin/geth -out /fixtures

FROM alpine:latest
WORKDIR /pbt-artifacts
COPY shims ./shims
COPY --from=builder /pbt-artifacts/pbt-artifacts ./pbt-artifacts
COPY --from=fixtures /fixtures ./fixtures

# One tar: ninety files uploaded one form-part each is slower than one tar
# untarred by the shim.
RUN tar -cf fixtures.tar -C fixtures genesis.json manifest.json valid preimages snapshot

ENTRYPOINT ["./pbt-artifacts"]
