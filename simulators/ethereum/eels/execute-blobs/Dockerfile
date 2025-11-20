# Builds and runs the EELS (execution-specs) consume engine simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default git-ref/fork
ARG branch=""
ARG fork=Osaka
ENV FORK=${fork}

## Clone and install EELS
RUN apt-get update && apt-get install -y git

# Clone repo and use the default branch if none specified
RUN git clone --depth 1 https://github.com/ethereum/execution-specs.git && \
    cd execution-specs && \
    if [ -n "$branch" ]; then \
        git fetch --depth 1 origin "$branch" && \
        git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs/packages/testing
RUN uv sync

## Define `consume engine` entry point using the local fixtures
ENTRYPOINT uv run execute hive --fork=$FORK -v -m blob_transaction_test
