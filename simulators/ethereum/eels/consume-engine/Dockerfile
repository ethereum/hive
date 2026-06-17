# Builds and runs the EELS (execution-specs) consume engine simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim AS base

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
RUN uv sync

# Optionally bake pre-staged fixtures into the image so `consume` reads them
# locally instead of downloading. A caller that stages ./fixtures.tar.gz and
# passes fixtures=/fixtures gets the local fixtures; otherwise FIXTURES stays a
# release/URL and consume downloads as before. Extraction runs in a throwaway
# stage so the tarball never lands in the final image (no size doubling); the
# `Dockerfile` anchor keeps COPY from failing when no tarball is staged.
FROM base AS fixtures-stage
COPY Dockerfile fixtures.tar.gz* /staged/
RUN mkdir -p /fixtures && \
    if [ -f /staged/fixtures.tar.gz ]; then tar -xzf /staged/fixtures.tar.gz -C /fixtures; fi

FROM base
COPY --from=fixtures-stage /fixtures /fixtures

# Cache the fixtures. This is done to avoid re-downloading the fixtures every time
# the container starts.
# If newer version of the fixtures is needed, the image needs to be rebuilt.
# Use `--docker.nocache` flag to force rebuild.
RUN uv run consume cache --input "$FIXTURES"


## Define `consume engine` entry point using the local fixtures
# TODO: remove when fixed on nimbus-el side.
ARG disable_strict_exception_matching=nimbus-el
ENV DISABLE_STRICT_EXCEPTION_MATCHING=${disable_strict_exception_matching}
ENTRYPOINT uv run consume engine -v --input "$FIXTURES" --disable-strict-exception-matching "$DISABLE_STRICT_EXCEPTION_MATCHING"
