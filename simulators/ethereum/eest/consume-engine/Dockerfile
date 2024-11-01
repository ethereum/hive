# Builds and runs the EEST (execution-spec-tests) consume engine simulator
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/ethereum/execution-spec-tests.git --branch main --single-branch --depth 1
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

## Define `consume engine` entry point using the local fixtures
ENTRYPOINT ["uv", "run", "consume", "engine", "-v", "--input", "latest-stable-release"]