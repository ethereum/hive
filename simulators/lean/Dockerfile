FROM rust:latest AS rust-builder

WORKDIR /workspace

ENV RUST_MIN_STACK=16777216

COPY ./Cargo.toml ./Cargo.toml
COPY ./Cargo.lock ./Cargo.lock
COPY ./hivesim-rs ./hivesim-rs
COPY ./simulators/lean ./simulators/lean

RUN cargo build -p lean-sim --release --locked \
    && mv target/release/lean-sim /lean-sim-bin \
    && cargo clean \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

FROM python:3.12-slim AS builder-devnet3

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

RUN apt-get update && apt-get install -y --no-install-recommends \
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

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain nightly \
    && rustup default nightly

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app/devnet3

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet3_tag}

RUN uv sync --frozen

FROM python:3.12-slim AS builder-devnet4

ARG helper_github=leanEthereum/leanSpec
ARG devnet4_tag=ad9a3226

RUN apt-get update && apt-get install -y --no-install-recommends \
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

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain nightly \
    && rustup default nightly

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

WORKDIR /app/devnet4

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet4_tag}

RUN uv sync --frozen

FROM python:3.12-slim

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d
ARG devnet4_tag=ad9a3226
ARG devnet3_keys_tag=leanSpec-ad9a3226
ARG devnet4_keys_tag=leanSpec-ad9a3226

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=rust-builder /lean-sim-bin /usr/bin/lean-sim
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
    LEAN_SPEC_GITHUB=${helper_github} \
    LEAN_SPEC_DEVNET3_ROOT=/app/devnet3 \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4 \
    LEAN_SPEC_DEVNET3_TAG=${devnet3_tag} \
    LEAN_SPEC_DEVNET4_TAG=${devnet4_tag} \
    LEAN_SPEC_DEVNET3_KEYS_DIR=/app/hive/prod_scheme_devnet3 \
    LEAN_SPEC_DEVNET4_KEYS_DIR=/app/hive/prod_scheme_devnet4

COPY ./simulators/lean/helper/leanspec-client.sh /app/hive/leanspec-client.sh
COPY ./simulators/lean/helper/lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY ./simulators/lean/helper/prepare_lean_client_assets.py /app/hive/prepare_lean_client_assets.py
COPY ./simulators/lean/config/lean-devnets.txt /app/hive/lean-devnets.txt

RUN mkdir -p /app/hive/prod_scheme_devnet3 /app/hive/prod_scheme_devnet4 /tmp/leansig \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet3_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet3.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet3.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet3/ \
    && rm -rf /tmp/leansig/prod_scheme \
    && curl -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet4_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet4.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet4.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet4/ \
    && rm -rf /tmp/leansig \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-sim\nhelper repo: %s\nhelper devnet3: %s\nhelper devnet4: %s\n" \
        "$helper_github" \
        "$devnet3_tag" \
        "$devnet4_tag" > /version.txt

EXPOSE 5052 5053 9001/udp

ENTRYPOINT ["/usr/bin/lean-sim"]
