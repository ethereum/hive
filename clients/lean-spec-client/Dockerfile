FROM python:3.12-slim AS builder-devnet3

ARG github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    build-essential \
    git \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain nightly \
    && rustup default nightly

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app/devnet3

RUN git clone https://github.com/$github . \
    && git checkout $devnet3_tag

RUN uv sync --frozen

FROM python:3.12-slim AS builder-devnet4

ARG github=leanEthereum/leanSpec
ARG devnet4_tag=2a30f80994f642f9523b85aba936f2f2e5b17529

RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    build-essential \
    git \
    pkg-config \
    libssl-dev \
    && rm -rf /var/lib/apt/lists/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain nightly \
    && rustup default nightly

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app/devnet4

RUN git clone https://github.com/$github . \
    && git checkout $devnet4_tag

RUN uv sync --frozen

FROM python:3.12-slim

ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d
ARG devnet4_tag=2a30f80994f642f9523b85aba936f2f2e5b17529

RUN apt-get update && apt-get install -y --no-install-recommends \
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

COPY --from=builder-devnet4 /app/devnet4/.venv /app/devnet4/.venv
COPY --from=builder-devnet4 /app/devnet4/src /app/devnet4/src
COPY --from=builder-devnet4 /app/devnet4/packages /app/devnet4/packages
COPY --from=builder-devnet4 /app/devnet4/pyproject.toml /app/devnet4/pyproject.toml
COPY --from=builder-devnet4 /app/devnet4/uv.lock /app/devnet4/uv.lock
COPY --from=builder-devnet4 /app/devnet4/LICENSE /app/devnet4/LICENSE
COPY --from=builder-devnet4 /app/devnet4/README.md /app/devnet4/README.md

ENV LEAN_ENV=prod \
    LEAN_SPEC_DEVNET3_ROOT=/app/devnet3 \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4

USER root

COPY leanspec-client.sh /app/hive/leanspec-client.sh
COPY lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY devnet4_keys /app/hive/prod_scheme_devnet4

RUN mkdir -p /app/hive/prod_scheme_devnet3 /tmp/leansig \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/leanSpec-ad9a3226/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet3/

RUN ls -l /app/hive/prod_scheme_devnet4 \
    && test -f /app/hive/prod_scheme_devnet4/0.json \
    && test -f /app/hive/prod_scheme_devnet4/1.json \
    && test -f /app/hive/prod_scheme_devnet4/2.json
RUN rm -rf /tmp/leansig \
    && chown -R leanspec:leanspec /app/hive \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-spec-client devnet3: %s\nlean-spec-client devnet4: %s\n" \
        "$devnet3_tag" \
        "$devnet4_tag" > /version.txt

USER leanspec

EXPOSE 5052 9001/udp

ENTRYPOINT ["/app/hive/leanspec-client.sh"]
