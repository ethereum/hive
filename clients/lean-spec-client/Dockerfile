FROM python:3.12-slim-bookworm AS lean-spec-base

ARG github=leanEthereum/leanSpec
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    libssl-dev \
    pkg-config \
    && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain ${RUST_NIGHTLY_TOOLCHAIN} \
    && rustup default ${RUST_NIGHTLY_TOOLCHAIN}

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

FROM lean-spec-base AS builder-devnet4

ARG github=leanEthereum/leanSpec
ARG devnet4_tag=main
ENV LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL=https://api.github.com/repos/${github}/commits/${devnet4_tag}

WORKDIR /app/devnet4

ADD ${LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL} /tmp/devnet4-commit.json

RUN git clone https://github.com/$github . \
    && git checkout $devnet4_tag \
    && git rev-parse HEAD > /app/devnet4/.git-commit

RUN uv sync --frozen

FROM lean-spec-base AS builder-devnet5

ARG github=leanEthereum/leanSpec
ARG devnet5_tag=main
ENV LEAN_SPEC_DEVNET5_COMMIT_METADATA_URL=https://api.github.com/repos/${github}/commits/${devnet5_tag}

WORKDIR /app/devnet5

ADD ${LEAN_SPEC_DEVNET5_COMMIT_METADATA_URL} /tmp/devnet5-commit.json

RUN git clone https://github.com/$github . \
    && git checkout $devnet5_tag \
    && git rev-parse HEAD > /app/devnet5/.git-commit

RUN uv sync --frozen

FROM python:3.12-slim-bookworm

ARG devnet4_tag=main
ARG devnet5_tag=main
ARG devnet5_keys_tag=latest
ARG lean_spec_tests_github=ReamLabs/lean-spec-tests
ARG lean_spec_tests_ref=a3bf5d72e74e3bdf7634e5ee3a8154aabf0a8565
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12
ENV LEAN_SPEC_DEVNET5_KEYS_METADATA_URL=https://api.github.com/repos/leanEthereum/leansig-test-keys/releases/tags/${devnet5_keys_tag}
ENV LEAN_SPEC_TESTS_METADATA_URL=https://api.github.com/repos/${lean_spec_tests_github}/commits/${lean_spec_tests_ref}

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --shell /bin/bash leanspec

WORKDIR /app

COPY --from=builder-devnet4 /app/devnet4/.venv /app/devnet4/.venv
COPY --from=builder-devnet4 /app/devnet4/src /app/devnet4/src
COPY --from=builder-devnet4 /app/devnet4/packages /app/devnet4/packages
COPY --from=builder-devnet4 /app/devnet4/pyproject.toml /app/devnet4/pyproject.toml
COPY --from=builder-devnet4 /app/devnet4/uv.lock /app/devnet4/uv.lock
COPY --from=builder-devnet4 /app/devnet4/LICENSE /app/devnet4/LICENSE
COPY --from=builder-devnet4 /app/devnet4/README.md /app/devnet4/README.md
COPY --from=builder-devnet4 /app/devnet4/.git-commit /app/devnet4/.git-commit
COPY --from=builder-devnet5 /app/devnet5/.venv /app/devnet5/.venv
COPY --from=builder-devnet5 /app/devnet5/src /app/devnet5/src
COPY --from=builder-devnet5 /app/devnet5/packages /app/devnet5/packages
COPY --from=builder-devnet5 /app/devnet5/pyproject.toml /app/devnet5/pyproject.toml
COPY --from=builder-devnet5 /app/devnet5/uv.lock /app/devnet5/uv.lock
COPY --from=builder-devnet5 /app/devnet5/LICENSE /app/devnet5/LICENSE
COPY --from=builder-devnet5 /app/devnet5/README.md /app/devnet5/README.md
COPY --from=builder-devnet5 /app/devnet5/.git-commit /app/devnet5/.git-commit

ENV LEAN_ENV=prod \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4 \
    LEAN_SPEC_DEVNET5_ROOT=/app/devnet5

USER root

COPY leanspec-client.sh /app/hive/leanspec-client.sh
COPY lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY lean_spec_runtime.py /app/hive/lean_spec_runtime.py

ADD ${LEAN_SPEC_TESTS_METADATA_URL} /tmp/lean-spec-tests-commit.json
ADD ${LEAN_SPEC_DEVNET5_KEYS_METADATA_URL} /tmp/devnet5-keys-release.json

RUN mkdir -p /app/hive/prod_scheme_devnet4 /app/hive/prod_scheme_devnet5 /tmp/leansig /tmp/lean-spec-tests \
    && curl -fsSL \
        https://github.com/${lean_spec_tests_github}/archive/${lean_spec_tests_ref}.tar.gz \
        -o /tmp/lean-spec-tests/lean-spec-tests.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/lean-spec-tests/lean-spec-tests.tar.gz --strip-components=1 -C /tmp/lean-spec-tests \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/0.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/1.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/2.json /app/hive/prod_scheme_devnet4/

RUN rm -rf /tmp/leansig/prod_scheme \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/$devnet5_keys_tag/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet5.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet5.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet5/

RUN ls -l /app/hive/prod_scheme_devnet4 \
    && test -f /app/hive/prod_scheme_devnet4/0.json \
    && test -f /app/hive/prod_scheme_devnet4/1.json \
    && test -f /app/hive/prod_scheme_devnet4/2.json
RUN ls -l /app/hive/prod_scheme_devnet5 \
    && test -f /app/hive/prod_scheme_devnet5/0.json \
    && test -f /app/hive/prod_scheme_devnet5/1.json \
    && test -f /app/hive/prod_scheme_devnet5/2.json
RUN DEVNET4_COMMIT="$(cat /app/devnet4/.git-commit)" \
    && DEVNET5_COMMIT="$(cat /app/devnet5/.git-commit)" \
    && LEAN_SPEC_TESTS_COMMIT="$(python3 -c 'import json; print(json.load(open("/tmp/lean-spec-tests-commit.json"))["sha"])')" \
    && rm -rf /tmp/lean-spec-tests /tmp/lean-spec-tests-commit.json /tmp/leansig /tmp/devnet5-keys-release.json \
    && chown -R leanspec:leanspec /app/hive \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-spec-client devnet4: %s\nlean-spec-client devnet5: %s\ndevnet4 keys: %s@%s\nrust toolchain: %s\n" \
        "$DEVNET4_COMMIT" \
        "$DEVNET5_COMMIT" \
        "$lean_spec_tests_github" \
        "$LEAN_SPEC_TESTS_COMMIT" \
        "$RUST_NIGHTLY_TOOLCHAIN" > /version.txt

USER leanspec

EXPOSE 5052 9001/udp

ENTRYPOINT ["/app/hive/leanspec-client.sh"]
