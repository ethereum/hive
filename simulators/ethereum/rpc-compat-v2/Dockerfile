# This simulation runs JSON-RPC API tests.
FROM golang:1-alpine as builder
ARG GOPROXY
ENV GOPROXY=${GOPROXY}
ARG branch=main
ENV GIT_REF=${branch}

# Only toggle value below false/true (false: download tests from execution-apis, true: assume local tests folder exists)
ARG USE_LOCAL_TESTS_FOLDER=false

ENV USE_LOCAL_TESTS_FOLDER=${USE_LOCAL_TESTS_FOLDER}
RUN apk add --update git ca-certificates gcc musl-dev linux-headers
RUN set -eux; \
    mkdir -p /tests; \
    if [ "$USE_LOCAL_TESTS_FOLDER" = "false" ]; then \
            echo "Fetching tests from ethereum/execution-apis at ${GIT_REF}..."; \
            git init /execution-apis; \
            cd /execution-apis; \
            git remote add origin https://github.com/ethereum/execution-apis.git; \
            git fetch --depth 1 origin $GIT_REF; \
            git checkout FETCH_HEAD; \
            rm -rf /tests; \
            cp -r /execution-apis/tests /tests; \
    else \
        echo "Using local tests folder (./tests)"; \
    fi

# Build the simulator executable.
ADD . /source
WORKDIR /source
RUN go build -v .

# Build the simulator run container.
FROM alpine:latest
ADD . /source
WORKDIR /source
COPY --from=builder /source/rpc-compat-v2 .
COPY --from=builder /tests ./tests

ENTRYPOINT ["./rpc-compat-v2"]
