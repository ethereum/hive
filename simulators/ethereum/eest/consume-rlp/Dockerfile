# Builds and runs the EEST (execution-spec-tests) consume rlp simulator
FROM python:3.10-slim

## Default fixtures and branch
ARG fixtures=stable@latest
ENV INPUT=${fixtures}
ARG branch=main
ENV BRANCH=${branch}

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/ethereum/execution-spec-tests.git --branch "$BRANCH" --single-branch --depth 1
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$INPUT"

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT uv run consume rlp -v --input "$INPUT"
