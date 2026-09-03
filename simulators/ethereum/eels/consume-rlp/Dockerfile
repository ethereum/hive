# Builds and runs the EELS (execution-specs) consume rlp simulator
FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

## Default fixtures/git-ref
ARG fixtures=stable@latest
ENV FIXTURES=${fixtures}
ARG branch=""
# Optional token for authenticated GitHub clones; anonymous clones are
# rate-limited per IP and fail intermittently on shared CI runners. Pass only
# short-lived tokens (e.g. the Actions job token): the classic builder records
# build-arg values in the image history.
ARG github_token=""

## Clone and install EELS
RUN apt-get update && apt-get install -y git isal

# Clone repo and use the default branch if none specified
RUN if [ -n "$github_token" ]; then \
        export GIT_CONFIG_COUNT=1 \
            GIT_CONFIG_KEY_0="http.https://github.com/.extraheader" \
            GIT_CONFIG_VALUE_0="AUTHORIZATION: basic $(printf 'x-access-token:%s' "$github_token" | base64 -w0)"; \
    fi && \
    git clone --depth 1 https://github.com/ethereum/execution-specs.git && \
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

## Define `consume rlp` entry point using the local fixtures
ENTRYPOINT if [ "$FIXTURES" = /fixtures ] && [ -f /staged/fixtures.tar.gz ]; then \
        mkdir -p /fixtures && \
        igzip -d -c /staged/fixtures.tar.gz | \
            tar -xf - -C /fixtures --strip-components=1 \
                fixtures/.meta fixtures/blockchain_tests; \
    fi && \
    exec uv run --no-sync consume rlp -v --input "$FIXTURES"
