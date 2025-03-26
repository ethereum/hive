# Builds and runs the EEST (execution-spec-tests) consume rlp simulator
FROM ghcr.io/astral-sh/uv:python3.10-bookworm-slim

## Default fixtures and branch
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=main
ENV BRANCH=${branch}

## Clone and install EEST
RUN apt-get update && apt-get install -y git
RUN git clone https://github.com/ethereum/execution-spec-tests.git --branch "$BRANCH" --single-branch --depth 1
WORKDIR /execution-spec-tests
RUN uv sync

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$FIXTURES"

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT uv run consume rlp -v --input "$FIXTURES"
