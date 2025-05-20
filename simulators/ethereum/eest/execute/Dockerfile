# Builds and runs the EEST (execution-spec-tests) execute hive simulator
FROM ghcr.io/astral-sh/uv:python3.10-bookworm-slim

## Default fork/marker/git-ref
ARG fork=Prague
ENV FORK=${fork}
ARG marker_expr=transaction_test
ENV MARKER_EXPR=${marker_expr}
ARG branch=main
ENV GIT_REF=${branch} 
ARG keyword=
ENV KEYWORD=${keyword}

## Clone and install EEST
RUN apt-get update && apt-get install -y git

# Allow the user to specify a branch or commit to checkout
RUN git init execution-spec-tests && \
    cd execution-spec-tests && \
    git remote add origin https://github.com/ethereum/execution-spec-tests.git && \
    git fetch --depth 1 origin $GIT_REF && \
    git checkout FETCH_HEAD;

WORKDIR /execution-spec-tests
RUN uv sync

## Define `execute hive` entry point using the local fixtures
ENTRYPOINT uv run execute hive --fork "$FORK" -m "$MARKER_EXPR" -k "$KEYWORD" -v

