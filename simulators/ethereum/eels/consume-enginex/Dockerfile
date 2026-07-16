# Builds and runs the EELS (execution-specs) consume enginex simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=""

## Clone and install EELS
RUN apt-get update && apt-get install -y git isal

# Clone repo and use the default branch if none specified
RUN git clone --depth 1 https://github.com/ethereum/execution-specs.git && \
    cd execution-specs && \
    if [ -n "$branch" ]; then \
    git fetch --depth 1 origin "$branch" && \
    git checkout FETCH_HEAD; \
    fi

WORKDIR /execution-specs/packages/testing
RUN uv sync

# Optionally keep pre-staged fixtures compressed in the image. A caller that
# stages ./fixtures.tar.gz and passes fixtures=/fixtures gets the local fixtures;
# otherwise FIXTURES stays a release/URL and consume downloads as before. The
# `Dockerfile` anchor keeps COPY from failing when no tarball is staged.
COPY Dockerfile fixtures.tar.gz* /staged/
RUN rm -f /staged/Dockerfile && \
    if [ "$FIXTURES" != /fixtures ] || [ ! -f /staged/fixtures.tar.gz ]; then \
        rm -rf /staged && uv run --no-sync consume cache --input "$FIXTURES"; \
    fi


## Define `consume enginex` entry point using the local fixtures
# TODO: remove when fixed on nimbus-el side.
ARG disable_strict_exception_matching=nimbus-el
ENV DISABLE_STRICT_EXCEPTION_MATCHING=${disable_strict_exception_matching}
ENTRYPOINT if [ "$FIXTURES" = /fixtures ] && [ -f /staged/fixtures.tar.gz ]; then \
        mkdir -p /fixtures && \
        igzip -d -c /staged/fixtures.tar.gz | \
            tar -xf - -C /fixtures --strip-components=1 \
                fixtures/.meta fixtures/blockchain_tests_engine_x; \
    fi && \
    exec uv run --no-sync consume enginex -v --input "$FIXTURES" --disable-strict-exception-matching "$DISABLE_STRICT_EXCEPTION_MATCHING"
