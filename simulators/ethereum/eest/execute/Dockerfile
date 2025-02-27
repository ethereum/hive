# Builds and runs the EEST (execution-spec-tests) execute hive simulator
FROM python:3.10-slim

## Default branch
ARG branch=main
ENV BRANCH=${branch}
ARG fork=Prague
ENV FORK=${fork}

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone https://github.com/ethereum/execution-spec-tests.git --branch "$BRANCH" --single-branch --depth 1
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

## Define `execute hive` entry point
ENTRYPOINT uv run execute hive --fork "$FORK"
