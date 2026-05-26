FROM rust:1.94.0-bookworm AS rust-builder

WORKDIR /workspace

ENV CARGO_BUILD_JOBS=1 \
    CARGO_PROFILE_DEV_CODEGEN_UNITS=1 \
    CARGO_PROFILE_DEV_DEBUG=0 \
    CARGO_INCREMENTAL=0 \
    RUST_MIN_STACK=4294967296

COPY ./Cargo.toml ./Cargo.toml
COPY ./Cargo.lock ./Cargo.lock
COPY ./hivesim-rs ./hivesim-rs
COPY ./simulators/lean/Cargo.toml ./simulators/lean/
COPY ./simulators/lean/src ./simulators/lean/src

RUN for attempt in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do \
        cargo build -p lean-sim --locked \
        && break; \
        if [ "$attempt" = "15" ]; then exit 1; fi; \
        sleep 2; \
    done \
    && mv target/debug/lean-sim /lean-sim-bin \
    && strip /lean-sim-bin || true \
    && cargo clean \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

FROM python:3.12-slim-bookworm AS lean-spec-base

ARG helper_github=leanEthereum/leanSpec
ARG RUST_NIGHTLY_TOOLCHAIN=1.95.0

RUN for attempt in 1 2 3; do \
        rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
        && apt-get clean \
        && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
        && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
            build-essential \
            ca-certificates \
            curl \
            git \
            libssl-dev \
            pkg-config \
        && break; \
        if [ "$attempt" = "3" ]; then exit 1; fi; \
        sleep 2; \
    done \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    CARGO_BUILD_JOBS=1 \
    CARGO_INCREMENTAL=0 \
    CARGO_PROFILE_RELEASE_CODEGEN_UNITS=16 \
    CARGO_PROFILE_RELEASE_LTO=false \
    CARGO_PROFILE_RELEASE_OPT_LEVEL=1 \
    RUSTFLAGS="-Cdebuginfo=0" \
    RUST_MIN_STACK=4294967296 \
    PATH=/usr/local/cargo/bin:$PATH

RUN mkdir -p "${CARGO_HOME}" \
    && printf '%s\n' \
        '[profile.release.package."lean_compiler"]' \
        'opt-level = 0' \
        'codegen-units = 256' \
        '' \
        '[profile.release.package."lean_prover"]' \
        'codegen-units = 256' \
        > "${CARGO_HOME}/config.toml"

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs \
    | sh -s -- -y --profile minimal --default-toolchain ${RUST_NIGHTLY_TOOLCHAIN} \
    && rustup default ${RUST_NIGHTLY_TOOLCHAIN}

RUN printf '%s\n' \
        '#!/bin/sh' \
        'export RUST_MIN_STACK="${RUSTC_MIN_STACK:-34359738368}"' \
        'exec "$@"' \
        > /usr/local/bin/rustc-stack-wrapper \
    && chmod +x /usr/local/bin/rustc-stack-wrapper

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

FROM lean-spec-base AS builder-devnet3

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

WORKDIR /app/devnet3

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet3_tag} \
    && git rev-parse HEAD > /app/devnet3/.git-commit

RUN for attempt in 1 2 3 4 5 6 7 8 9 10; do \
        ulimit -s unlimited \
        && RUSTC_WRAPPER=/usr/local/bin/rustc-stack-wrapper \
            RUSTC_MIN_STACK=17179869184 \
            uv sync --frozen \
        && break; \
        if [ "$attempt" = "10" ]; then exit 1; fi; \
        sleep 2; \
    done

FROM lean-spec-base AS builder-devnet4

ARG helper_github=leanEthereum/leanSpec
ARG devnet4_tag=c460769ac6385812f193f5d61c178dc8df03d81e

WORKDIR /app/devnet4

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet4_tag} \
    && git rev-parse HEAD > /app/devnet4/.git-commit

RUN for attempt in 1 2 3 4 5 6 7 8 9 10; do \
        ulimit -s unlimited \
        && RUSTC_WRAPPER=/usr/local/bin/rustc-stack-wrapper \
            RUSTC_MIN_STACK=17179869184 \
            uv sync --frozen \
        && break; \
        if [ "$attempt" = "10" ]; then exit 1; fi; \
        sleep 2; \
    done

FROM python:3.12-slim-bookworm

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d
ARG devnet4_tag=c460769ac6385812f193f5d61c178dc8df03d81e
ARG devnet3_keys_tag=leanSpec-ad9a3226
ARG devnet4_keys_tag=latest
ARG lean_spec_fixtures_tag=latest
ARG RUST_NIGHTLY_TOOLCHAIN=1.95.0
ENV LEAN_SPEC_FIXTURES_METADATA_URL=https://api.github.com/repos/${helper_github}/releases/tags/${lean_spec_fixtures_tag}
ENV LEAN_SPEC_DEVNET4_KEYS_METADATA_URL=https://api.github.com/repos/leanEthereum/leansig-test-keys/releases/tags/${devnet4_keys_tag}

RUN for attempt in 1 2 3; do \
        rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
        && apt-get clean \
        && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
        && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
            ca-certificates \
            curl \
        && break; \
        if [ "$attempt" = "3" ]; then exit 1; fi; \
        sleep 2; \
    done \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/*

WORKDIR /app

COPY --from=rust-builder /lean-sim-bin /usr/bin/lean-sim
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
    LEAN_SPEC_GITHUB=${helper_github} \
    LEAN_SPEC_DEVNET3_ROOT=/app/devnet3 \
    LEAN_SPEC_DEVNET4_ROOT=/app/devnet4 \
    LEAN_SPEC_DEVNET3_KEYS_DIR=/app/hive/prod_scheme_devnet3 \
    LEAN_SPEC_DEVNET4_KEYS_DIR=/app/hive/prod_scheme_devnet4 \
    PYTHONFAULTHANDLER=1 \
    RUST_MIN_STACK=4294967296

COPY ./simulators/lean/helper/leanspec-client.sh /app/hive/leanspec-client.sh
COPY ./simulators/lean/helper/lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY ./clients/lean-spec-client/lean_spec_runtime.py /app/hive/lean_spec_runtime.py
COPY ./simulators/lean/helper/prepare_lean_client_assets.py /app/hive/prepare_lean_client_assets.py
COPY ./simulators/lean/config/lean-devnets.txt /app/hive/lean-devnets.txt

ADD ${LEAN_SPEC_FIXTURES_METADATA_URL} /tmp/lean-spec-fixtures-release.json
ADD ${LEAN_SPEC_DEVNET4_KEYS_METADATA_URL} /tmp/devnet4-keys-release.json

RUN mkdir -p /app/hive/prod_scheme_devnet3 /app/hive/prod_scheme_devnet4 /app/hive/lean-spec-tests /tmp/lean-spec-fixtures /tmp/leansig \
    && curl -4 -fsSL \
        https://github.com/${helper_github}/releases/download/${lean_spec_fixtures_tag}/fixtures-prod-scheme.tar.gz \
        -o /tmp/lean-spec-fixtures/fixtures-prod-scheme.tar.gz \
    && curl -4 -fsSL \
        https://github.com/${helper_github}/releases/download/${lean_spec_fixtures_tag}/fixtures-prod-scheme.tar.gz.sha256 \
        -o /tmp/lean-spec-fixtures/fixtures-prod-scheme.tar.gz.sha256 \
    && (cd /tmp/lean-spec-fixtures && sha256sum -c fixtures-prod-scheme.tar.gz.sha256) \
    && tar --warning=no-unknown-keyword -xzf /tmp/lean-spec-fixtures/fixtures-prod-scheme.tar.gz --strip-components=1 -C /app/hive/lean-spec-tests \
    && curl -4 -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet3_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet3.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet3.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
        prod_scheme/3.json \
        prod_scheme/4.json \
        prod_scheme/5.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet3/ \
    && rm -rf /tmp/leansig/prod_scheme \
    && curl -4 -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet4_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet4.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet4.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
        prod_scheme/3.json \
        prod_scheme/4.json \
        prod_scheme/5.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet4/ \
    && DEVNET3_COMMIT="$(cat /app/devnet3/.git-commit)" \
    && DEVNET4_COMMIT="$(cat /app/devnet4/.git-commit)" \
    && rm -rf /tmp/lean-spec-fixtures /tmp/lean-spec-fixtures-release.json /tmp/leansig /tmp/devnet4-keys-release.json \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-sim\nhelper repo: %s\nhelper devnet3: %s\nhelper devnet4: %s\nfixtures release: %s\nrust toolchain: %s\n" \
        "$helper_github" \
        "$DEVNET3_COMMIT" \
        "$DEVNET4_COMMIT" \
        "$lean_spec_fixtures_tag" \
        "$RUST_NIGHTLY_TOOLCHAIN" > /version.txt

# Fix missing IdentityKeypair.from_bytes in lean_spec
RUN <<'EOF'
python3 -c '
import sys

filepath = "/app/devnet4/src/lean_spec/subspecs/networking/transport/identity/keypair.py"
with open(filepath, "r") as f:
    content = f.read()

old_method = """    @classmethod
    def generate(cls) -> IdentityKeypair:
        \"\"\"
        Generate a new random secp256k1 keypair.

        Returns:
            A fresh identity keypair.
        \"\"\"
        private_key = ec.generate_private_key(ec.SECP256K1())
        public_key = Secp256k1PublicKey(_key=private_key.public_key())
        return cls(private_key=private_key, public_key=public_key)"""

new_method = """    @classmethod
    def generate(cls) -> IdentityKeypair:
        \"\"\"
        Generate a new random secp256k1 keypair.

        Returns:
            A fresh identity keypair.
        \"\"\"
        private_key = ec.generate_private_key(ec.SECP256K1())
        public_key = Secp256k1PublicKey(_key=private_key.public_key())
        return cls(private_key=private_key, public_key=public_key)

    @classmethod
    def from_bytes(cls, private_key_bytes: bytes) -> IdentityKeypair:
        \"\"\"
        Create an identity keypair from raw private key bytes.

        Args:
            private_key_bytes: 32-byte secp256k1 private key.

        Returns:
            Identity keypair derived from the private key.
        \"\"\"
        private_value = int.from_bytes(private_key_bytes, "big")
        private_key = ec.derive_private_key(private_value, ec.SECP256K1())
        public_key = Secp256k1PublicKey(_key=private_key.public_key())
        return cls(private_key=private_key, public_key=public_key)"""

if old_method in content:
    content = content.replace(old_method, new_method)
    with open(filepath, "w") as f:
        f.write(content)
    print("Fixed IdentityKeypair.from_bytes")
else:
    print("Could not find method to patch")
    sys.exit(1)
'
EOF

EXPOSE 5052 5053 9001/udp

ENTRYPOINT ["/usr/bin/lean-sim"]
