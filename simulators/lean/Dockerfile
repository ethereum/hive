FROM rust:bookworm AS rust-builder

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

FROM lean-spec-base AS builder-devnet3

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

WORKDIR /app/devnet3

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet3_tag} \
    && git rev-parse HEAD > /app/devnet3/.git-commit

RUN uv sync --frozen

FROM lean-spec-base AS builder-devnet4

ARG helper_github=leanEthereum/leanSpec
ARG devnet4_tag=main
ENV LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL=https://api.github.com/repos/${helper_github}/commits/${devnet4_tag}

WORKDIR /app/devnet4

ADD ${LEAN_SPEC_DEVNET4_COMMIT_METADATA_URL} /tmp/devnet4-commit.json

RUN git clone https://github.com/${helper_github} . \
    && git checkout ${devnet4_tag} \
    && git rev-parse HEAD > /app/devnet4/.git-commit

RUN uv sync --frozen

FROM python:3.12-slim-bookworm

ARG helper_github=leanEthereum/leanSpec
ARG devnet3_tag=be853180d21aa36d6401b8c1541aa6fcaad5008d
ARG devnet4_tag=main
ARG devnet3_keys_tag=leanSpec-ad9a3226
ARG devnet4_keys_tag=latest
ARG lean_spec_tests_ref=master
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12
ENV LEAN_SPEC_DEVNET4_KEYS_METADATA_URL=https://api.github.com/repos/leanEthereum/leansig-test-keys/releases/tags/${devnet4_keys_tag}

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y -o Acquire::Retries=3 --no-install-recommends \
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
    LEAN_SPEC_DEVNET4_KEYS_DIR=/app/hive/prod_scheme_devnet4

COPY ./simulators/lean/helper/leanspec-client.sh /app/hive/leanspec-client.sh
COPY ./simulators/lean/helper/lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY ./clients/lean-spec-client/lean_spec_runtime.py /app/hive/lean_spec_runtime.py
COPY ./simulators/lean/helper/prepare_lean_client_assets.py /app/hive/prepare_lean_client_assets.py
COPY ./simulators/lean/config/lean-devnets.txt /app/hive/lean-devnets.txt

ADD ${LEAN_SPEC_DEVNET4_KEYS_METADATA_URL} /tmp/devnet4-keys-release.json

RUN mkdir -p /app/hive/prod_scheme_devnet3 /app/hive/prod_scheme_devnet4 /app/hive/lean-spec-tests /tmp/leansig \
    && curl -4 -fsSL \
        https://github.com/ReamLabs/lean-spec-tests/archive/${lean_spec_tests_ref}.tar.gz \
        | tar --warning=no-unknown-keyword -xz --strip-components=1 -C /app/hive/lean-spec-tests \
    && curl -4 -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet3_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet3.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet3.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet3/ \
    && rm -rf /tmp/leansig/prod_scheme \
    && curl -4 -fsSL \
        https://github.com/leanEthereum/leansig-test-keys/releases/download/${devnet4_keys_tag}/prod_scheme.tar.gz \
        -o /tmp/leansig/prod_scheme_devnet4.tar.gz \
    && tar --warning=no-unknown-keyword -xzf /tmp/leansig/prod_scheme_devnet4.tar.gz -C /tmp/leansig \
        prod_scheme/0.json \
        prod_scheme/1.json \
        prod_scheme/2.json \
    && cp /tmp/leansig/prod_scheme/*.json /app/hive/prod_scheme_devnet4/ \
    && DEVNET3_COMMIT="$(cat /app/devnet3/.git-commit)" \
    && DEVNET4_COMMIT="$(cat /app/devnet4/.git-commit)" \
    && rm -rf /tmp/leansig /tmp/devnet4-keys-release.json \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-sim\nhelper repo: %s\nhelper devnet3: %s\nhelper devnet4: %s\nrust toolchain: %s\n" \
        "$helper_github" \
        "$DEVNET3_COMMIT" \
        "$DEVNET4_COMMIT" \
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
