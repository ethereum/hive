# Builds and runs the EELS (execution-specs) `consume reorg` simulator.
#
# Consumes `blockchain_test_engine_reorg` fixtures: a DAG of engine payloads
# plus an ordered list of Engine API / JSON-RPC steps with expected outcomes.
# One fresh client per fixture; pass/fail per test; no runtime knobs.
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=""

## Source repository. Override to build from a fork, e.g. while the fixture
## format is still under review:
##   --sim.buildarg repo=https://github.com/<user>/execution-specs
##   --sim.buildarg branch=<branch>
ARG repo=https://github.com/ethereum/execution-specs

## Clone and install EELS
RUN apt-get update && apt-get install -y git isal

# Clone repo and use the default branch if none specified
RUN git clone --depth 1 "$repo" execution-specs && \
    cd execution-specs && \
    if [ -n "$branch" ]; then \
    git fetch --depth 1 origin "$branch" && \
    git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs/packages/testing
RUN uv sync

# Optionally keep pre-staged fixtures compressed in the image. A caller that
# stages ./fixtures.tar.gz and passes fixtures=/fixtures gets the local
# fixtures; otherwise FIXTURES stays a release/URL and consume downloads it.
# The `Dockerfile` anchor keeps COPY from failing when no tarball is staged.
COPY Dockerfile fixtures.tar.gz* /staged/
RUN rm -f /staged/Dockerfile && \
    if [ "$FIXTURES" != /fixtures ] || [ ! -f /staged/fixtures.tar.gz ]; then \
        rm -rf /staged && uv run --no-sync consume cache --input "$FIXTURES"; \
    fi

## Define `consume reorg` entry point using the local fixtures
ENTRYPOINT if [ "$FIXTURES" = /fixtures ] && [ -f /staged/fixtures.tar.gz ]; then \
        mkdir -p /fixtures && \
        igzip -d -c /staged/fixtures.tar.gz | \
            tar -xf - -C /fixtures --strip-components=1 \
                fixtures/.meta fixtures/blockchain_tests_engine_reorg; \
    fi && \
    exec uv run --no-sync consume reorg -v --input "$FIXTURES"
