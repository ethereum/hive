# Builds and runs the EEST (execution-spec-tests) consume sync simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=forks/osaka
ENV GIT_REF=${branch}

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

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$FIXTURES"

## Define `consume sync` entry point using the local fixtures
ENTRYPOINT uv run consume sync -v --input "$FIXTURES"
