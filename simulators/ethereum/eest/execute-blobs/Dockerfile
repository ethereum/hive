# Builds and runs the EEST (execution-spec-tests) consume engine simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default git-ref/fork
ARG branch=forks/osaka
ENV GIT_REF=${branch} 
ARG fork=Osaka
ENV FORK=${fork}

## Clone and install EEST
RUN apt-get update && apt-get install -y git

# Allow the user to specify a branch or commit to checkout
RUN git init execution-specs && \
    cd execution-specs && \
    git remote add origin https://github.com/ethereum/execution-specs.git && \
    git fetch --depth 1 origin $GIT_REF && \
    git checkout FETCH_HEAD;

WORKDIR /execution-specs/packages/tests
RUN uv sync

## Define `consume engine` entry point using the local fixtures
ENTRYPOINT uv run execute hive --fork=$FORK -v -m blob_transaction_test
