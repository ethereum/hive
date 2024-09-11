# Builds and runs the EEST (execution-spec-tests) execute
FROM python:3.10-slim

## Install dependencies
RUN apt-get update && \
    apt-get install -y git wget tar && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

## Clone and install EEST
RUN git clone  -b fill-execute-modes-updates https://github.com/spencer-tb/execution-spec-tests.git
WORKDIR execution-spec-tests
RUN pip install uv && uv sync

## Define `execute` entry point using python tests
ENTRYPOINT ["uv", "run", "execute", "--hive-mode", "--fork", "Prague"]
