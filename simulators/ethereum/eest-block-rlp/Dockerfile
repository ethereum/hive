# Builds and runs the EEST (execution-spec-tests) consume rlp simulator
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/ethereum/execution-spec-tests.git --depth 1
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT ["uv", "run", "consume", "rlp", "-v", "--input", "https://github.com/ethereum/execution-spec-tests/releases/download/pectra-devnet-3%40v1.5.0/fixtures_pectra-devnet-3.tar.gz"]

