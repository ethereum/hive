FROM python:3.12-slim AS builder

ARG github=leanEthereum/leanSpec
ARG tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

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

WORKDIR /app

RUN git clone https://github.com/$github . \
    && git checkout $tag

RUN uv sync --frozen

FROM python:3.12-slim

ARG tag=be853180d21aa36d6401b8c1541aa6fcaad5008d

RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    && rm -rf /var/lib/apt/lists/*

COPY --from=ghcr.io/astral-sh/uv:latest /uv /usr/local/bin/uv

RUN useradd --create-home --shell /bin/bash leanspec

WORKDIR /app

COPY --from=builder /app/.venv /app/.venv
COPY --from=builder /app/src /app/src
COPY --from=builder /app/packages /app/packages
COPY --from=builder /app/pyproject.toml /app/pyproject.toml
COPY --from=builder /app/uv.lock /app/uv.lock
COPY --from=builder /app/LICENSE /app/LICENSE
COPY --from=builder /app/README.md /app/README.md

ENV LEAN_ENV=prod \
    VIRTUAL_ENV=/app/.venv \
    PATH="/app/.venv/bin:$PATH"

USER root

COPY download_leanspec_prod_keys.py /app/hive/download_leanspec_prod_keys.py
COPY leanspec-client.sh /app/hive/leanspec-client.sh
COPY lean_spec_client_runner.py /app/hive/lean_spec_client_runner.py
COPY prepare_leanspec_assets.py /app/hive/prepare_leanspec_assets.py

RUN python /app/hive/download_leanspec_prod_keys.py \
    && chown -R leanspec:leanspec /app /app/hive \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-spec-client %s\n" "$tag" > /version.txt

USER leanspec

EXPOSE 5052 9001/udp

ENTRYPOINT ["/app/hive/leanspec-client.sh"]
