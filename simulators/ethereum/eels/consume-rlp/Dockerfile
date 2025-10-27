# Builds and runs the EELS (execution-specs) consume rlp simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=""

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

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$FIXTURES"
RUN uv sync

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT uv run consume rlp -v --input "$FIXTURES"
