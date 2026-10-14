# This simulation runs JSON-RPC API tests.
FROM golang:1-alpine AS builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
ARG branch=main
ENV GIT_REF=${branch}

RUN apk add --update git ca-certificates make

# Clone the tests repo.
# Allow the user to specify a branch or commit to checkout
RUN git init /execution-apis && \
    cd /execution-apis && \
    git remote add origin https://github.com/ethereum/execution-apis.git && \
    git fetch --depth 1 origin $GIT_REF && \
    git checkout FETCH_HEAD;

# Generate openrpc.json from the spec source. It is gitignored in
# execution-apis (built by `make build`), so it is not present in the clone.
# speconly tests validate responses against this schema.
RUN cd /execution-apis && make build

# To run local tests, copy the directory into the same as the simulator and
# uncomment the line below
# ADD tests /execution-apis/tests

# To run against a local spec, copy refs-openrpc.json next to the simulator and
# uncomment the line below (overrides the generated spec above).
# ADD refs-openrpc.json /execution-apis/refs-openrpc.json

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/rpc-compat .
COPY --from=builder /execution-apis/tests ./tests
COPY --from=builder /execution-apis/refs-openrpc.json ./refs-openrpc.json

ENTRYPOINT ["./rpc-compat"]
