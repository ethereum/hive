# Builds and runs the EELS (execution-specs) execute-blobs simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default git-ref/fork
ARG branch=""
# Optional token for authenticated GitHub clones; anonymous clones are
# rate-limited per IP and fail intermittently on shared CI runners. Pass only
# short-lived tokens (e.g. the Actions job token): the classic builder records
# build-arg values in the image history.
ARG github_token=""
ARG fork=Osaka
ENV FORK=${fork}

## Clone and install EELS
RUN apt-get update && apt-get install -y git

# Clone repo and use the default branch if none specified
RUN if [ -n "$github_token" ]; then \
        export GIT_CONFIG_COUNT=1 \
            GIT_CONFIG_KEY_0="http.https://github.com/.extraheader" \
            GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$github_token" | base64 -w0)"; \
    fi && \
    git clone --depth 1 https://github.com/ethereum/execution-specs.git && \
    cd execution-specs && \
    if [ -n "$branch" ]; then \
        git fetch --depth 1 origin "$branch" && \
        git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs/
RUN uv sync

## Define `execute hive` entry point for the blob transaction tests
# --no-sync skips re-syncing the environment on container start; the image
# build already ran `uv sync`.
ENTRYPOINT exec uv run --no-sync execute hive --fork=$FORK -v -m blob_transaction_test
