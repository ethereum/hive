# Builds and runs the EELS (execution-specs) execute-blobs simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default git-ref/fork
ARG branch=""
ARG fork=Osaka
ENV FORK=${fork}

## Clone and install EELS
RUN apt-get update && apt-get install -y git cmake g++ libclang-dev

# Clone repo and use the default branch if none specified
RUN git clone --depth 1 https://github.com/ethereum/execution-specs.git && \
    cd execution-specs && \
    if [ -n "$branch" ]; then \
        git fetch --depth 1 origin "$branch" && \
        git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs/
RUN uv sync --all-extras

## Define `execute hive` entry point for the blob transaction tests
# --no-sync skips re-syncing the environment on container start; the image
# build already ran `uv sync --all-extras`.
ENTRYPOINT exec uv run --no-sync execute hive --fork=$FORK -v -m blob_transaction_test
