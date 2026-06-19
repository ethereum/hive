FROM rust:1.92.0-bookworm AS rust-builder

WORKDIR /workspace

ENV RUST_MIN_STACK=67108864

COPY ./Cargo.toml ./Cargo.toml
COPY ./Cargo.lock ./Cargo.lock
COPY ./hivesim-rs ./hivesim-rs
COPY ./simulators/lean ./simulators/lean

RUN cargo build -p lean-sim --release --locked \
    && mv target/release/lean-sim /lean-sim-bin \
    && cargo clean \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

FROM python:3.12-slim-bookworm AS lean-spec-base

ARG helper_github=leanEthereum/leanSpec
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

ARG helper_github=leanEthereum/leanSpec
ARG devnet4_tag=db21cc24cbeeb78a91c42a713089fe0c845eb648
ENV LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL=https://api.github.com/repos/${helper_github}/commits/${devnet4_tag}

WORKDIR /app/devnet4

ADD ${LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL} /tmp/devnet4-commit.json

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet4_tag} \
    && git rev-parse HEAD > /app/devnet4/.git-commit

RUN uv sync --frozen

FROM lean-spec-base AS builder-devnet5

ARG helper_github=leanEthereum/leanSpec
ARG devnet5_tag=main
ENV LEAN_SPEC_DEVNET5_COMMIT_METADATA_URL=https://api.github.com/repos/${helper_github}/commits/${devnet5_tag}

WORKDIR /app/devnet5

ADD ${LEAN_SPEC_DEVNET5_COMMIT_METADATA_URL} /tmp/devnet5-commit.json

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet5_tag} \
    && git rev-parse HEAD > /app/devnet5/.git-commit

RUN uv sync --frozen

FROM python:3.12-slim-bookworm

ARG helper_github=leanEthereum/leanSpec
ARG devnet4_tag=db21cc24cbeeb78a91c42a713089fe0c845eb648
ARG devnet5_tag=main
ARG devnet5_keys_tag=latest
ARG lean_spec_fixtures_tag=latest
ARG lean_spec_tests_github=ReamLabs/lean-spec-tests
ARG lean_spec_tests_ref=a3bf5d72e74e3bdf7634e5ee3a8154aabf0a8565
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12
ENV LEAN_SPEC_FIXTURES_METADATA_URL=https://api.github.com/repos/${helper_github}/releases/tags/${lean_spec_fixtures_tag}
ENV LEAN_SPEC_DEVNET5_KEYS_METADATA_URL=https://api.github.com/repos/leanEthereum/leansig-test-keys/releases/tags/${devnet5_keys_tag}
ENV LEAN_SPEC_TESTS_METADATA_URL=https://api.github.com/repos/${lean_spec_tests_github}/commits/${lean_spec_tests_ref}

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=rust-builder /lean-sim-bin /usr/bin/lean-sim
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
    LEAN_SPEC_GITHUB=${helper_github} \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4 \
    LEAN_SPEC_DEVNET5_ROOT=/app/devnet5 \
    LEAN_SPEC_DEVNET4_KEYS_DIR=/app/hive/prod_scheme_devnet4 \
    LEAN_SPEC_DEVNET5_KEYS_DIR=/app/hive/prod_scheme_devnet5 \
    PYTHONFAULTHANDLER=1 \
    RUST_MIN_STACK=33554432

COPY ./simulators/lean/helper/leanspec-client.sh /app/hive/leanspec-client.sh
COPY ./simulators/lean/helper/lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY ./clients/lean-spec-client/lean_spec_runtime.py /app/hive/lean_spec_runtime.py
COPY ./simulators/lean/helper/prepare_lean_client_assets.py /app/hive/prepare_lean_client_assets.py
COPY ./simulators/lean/config/lean-devnets.txt /app/hive/lean-devnets.txt

ADD ${LEAN_SPEC_TESTS_METADATA_URL} /tmp/lean-spec-tests-commit.json
ADD ${LEAN_SPEC_FIXTURES_METADATA_URL} /tmp/devnet5-lean-spec-fixtures-release.json
ADD ${LEAN_SPEC_DEVNET5_KEYS_METADATA_URL} /tmp/devnet5-keys-release.json

RUN mkdir -p \
        /app/hive/prod_scheme_devnet4 \
        /app/hive/prod_scheme_devnet5 \
        /app/hive/lean-spec-tests-devnet4 \
        /app/hive/lean-spec-tests-devnet5 \
        /tmp/lean-spec-tests \
        /tmp/devnet5-lean-spec-fixtures \
        /tmp/leansig \
    && curl -4 -fsSL \
        https://github.com/${lean_spec_tests_github}/archive/${lean_spec_tests_ref}.tar.gz \
        -o /tmp/lean-spec-tests/lean-spec-tests.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/lean-spec-tests/lean-spec-tests.tar.gz --strip-components=1 -C /tmp/lean-spec-tests \
    && cp -a /tmp/lean-spec-tests/consensus /app/hive/lean-spec-tests-devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/0.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/1.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/2.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/3.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/4.json /app/hive/prod_scheme_devnet4/ \
    && cp /tmp/lean-spec-tests/keys/prod_scheme/5.json /app/hive/prod_scheme_devnet4/ \
    && curl -4 -fsSL \
        https://github.com/${helper_github}/releases/download/${lean_spec_fixtures_tag}/fixtures-prod-scheme.tar.gz \
        -o /tmp/devnet5-lean-spec-fixtures/fixtures-prod-scheme.tar.gz \
    && curl -4 -fsSL \
        https://github.com/${helper_github}/releases/download/${lean_spec_fixtures_tag}/fixtures-prod-scheme.tar.gz.sha256 \
        -o /tmp/devnet5-lean-spec-fixtures/fixtures-prod-scheme.tar.gz.sha256 \
    && (cd /tmp/devnet5-lean-spec-fixtures && sha256sum -c fixtures-prod-scheme.tar.gz.sha256) \
    && tar --warning=no-unknown-keyword -xzf /tmp/devnet5-lean-spec-fixtures/fixtures-prod-scheme.tar.gz --strip-components=1 -C /app/hive/lean-spec-tests-devnet5 \
    && curl -4 -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet5_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet5.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet5.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
        prod_scheme/3.json \
        prod_scheme/4.json \
        prod_scheme/5.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet5/ \
    && DEVNET4_COMMIT="$(cat /app/devnet4/.git-commit)" \
    && DEVNET5_COMMIT="$(cat /app/devnet5/.git-commit)" \
    && LEAN_SPEC_TESTS_COMMIT="$(python3 -c 'import json; print(json.load(open("/tmp/lean-spec-tests-commit.json"))["sha"])')" \
    && rm -rf /tmp/lean-spec-tests /tmp/lean-spec-tests-commit.json /tmp/devnet5-lean-spec-fixtures /tmp/devnet5-lean-spec-fixtures-release.json /tmp/leansig /tmp/devnet5-keys-release.json \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-sim\nhelper repo: %s\nhelper devnet4: %s\nhelper devnet5: %s\ndevnet4 spec assets: %s@%s\ndevnet5 fixtures release: %s\nrust toolchain: %s\n" \
        "$helper_github" \
        "$DEVNET4_COMMIT" \
        "$DEVNET5_COMMIT" \
        "$lean_spec_tests_github" \
        "$LEAN_SPEC_TESTS_COMMIT" \
        "$lean_spec_fixtures_tag" \
        "$RUST_NIGHTLY_TOOLCHAIN" > /version.txt

EXPOSE 5052 5053 9001/udp

ENTRYPOINT ["/usr/bin/lean-sim"]
