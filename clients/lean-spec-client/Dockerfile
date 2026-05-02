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

FROM lean-spec-base AS builder-devnet3

ARG github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

WORKDIR /app/devnet3

RUN git clone https://github.com/$github . \
    && git checkout $devnet3_tag \
    && git rev-parse HEAD > /app/devnet3/.git-commit

RUN uv sync --frozen

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

FROM python:3.12-slim-bookworm

ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d
ARG devnet4_tag=main
ARG devnet3_keys_tag=leanSpec-ad9a3226
ARG devnet4_keys_tag=latest
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12
ENV LEAN_SPEC_DEVNET4_KEYS_METADATA_URL=https://api.github.com/repos/leanEthereum/leansig-test-keys/releases/tags/${devnet4_keys_tag}

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

RUN useradd --create-home --shell /bin/bash leanspec

WORKDIR /app

COPY --from=builder-devnet3 /app/devnet3/.venv /app/devnet3/.venv
COPY --from=builder-devnet3 /app/devnet3/src /app/devnet3/src
COPY --from=builder-devnet3 /app/devnet3/packages /app/devnet3/packages
COPY --from=builder-devnet3 /app/devnet3/pyproject.toml /app/devnet3/pyproject.toml
COPY --from=builder-devnet3 /app/devnet3/uv.lock /app/devnet3/uv.lock
COPY --from=builder-devnet3 /app/devnet3/LICENSE /app/devnet3/LICENSE
COPY --from=builder-devnet3 /app/devnet3/README.md /app/devnet3/README.md
COPY --from=builder-devnet3 /app/devnet3/.git-commit /app/devnet3/.git-commit

COPY --from=builder-devnet4 /app/devnet4/.venv /app/devnet4/.venv
COPY --from=builder-devnet4 /app/devnet4/src /app/devnet4/src
COPY --from=builder-devnet4 /app/devnet4/packages /app/devnet4/packages
COPY --from=builder-devnet4 /app/devnet4/pyproject.toml /app/devnet4/pyproject.toml
COPY --from=builder-devnet4 /app/devnet4/uv.lock /app/devnet4/uv.lock
COPY --from=builder-devnet4 /app/devnet4/LICENSE /app/devnet4/LICENSE
COPY --from=builder-devnet4 /app/devnet4/README.md /app/devnet4/README.md
COPY --from=builder-devnet4 /app/devnet4/.git-commit /app/devnet4/.git-commit

ENV LEAN_ENV=prod \
    LEAN_SPEC_DEVNET3_ROOT=/app/devnet3 \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4

USER root

COPY leanspec-client.sh /app/hive/leanspec-client.sh
COPY lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY lean_spec_runtime.py /app/hive/lean_spec_runtime.py

ADD ${LEAN_SPEC_DEVNET4_KEYS_METADATA_URL} /tmp/devnet4-keys-release.json

RUN mkdir -p /app/hive/prod_scheme_devnet3 /app/hive/prod_scheme_devnet4 /tmp/leansig \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/$devnet3_keys_tag/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet3.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet3.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet3/ \
    && rm -rf /tmp/leansig/prod_scheme \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/$devnet4_keys_tag/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet4.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet4.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet4/

RUN ls -l /app/hive/prod_scheme_devnet4 \
    && test -f /app/hive/prod_scheme_devnet4/0.json \
    && test -f /app/hive/prod_scheme_devnet4/1.json \
    && test -f /app/hive/prod_scheme_devnet4/2.json
RUN DEVNET3_COMMIT="$(cat /app/devnet3/.git-commit)" \
    && DEVNET4_COMMIT="$(cat /app/devnet4/.git-commit)" \
    && rm -rf /tmp/leansig /tmp/devnet4-keys-release.json \
    && chown -R leanspec:leanspec /app/hive \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-spec-client devnet3: %s\nlean-spec-client devnet4: %s\nrust toolchain: %s\n" \
        "$DEVNET3_COMMIT" \
        "$DEVNET4_COMMIT" \
        "$RUST_NIGHTLY_TOOLCHAIN" > /version.txt

USER leanspec

EXPOSE 5052 9001/udp

ENTRYPOINT ["/app/hive/leanspec-client.sh"]
