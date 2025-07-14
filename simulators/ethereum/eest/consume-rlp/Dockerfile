# Builds and runs the EEST (execution-spec-tests) consume rlp simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=main
ENV GIT_REF=${branch}

## Install EEST
## TODO: Remove this line once EEST is published to PyPI
RUN apt-get update && apt-get install -y git

WORKDIR /execution-spec-tests
RUN uv venv && uv pip install git+https://github.com/ethereum/execution-spec-tests.git@$GIT_REF

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$FIXTURES"

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT uv run consume rlp -v --input "$FIXTURES"
