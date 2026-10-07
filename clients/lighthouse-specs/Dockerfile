## Lighthouse consensus-spec-tests runner. Built from source at container
## startup so we can pin to any tag/branch/SHA via HIVE_CL_SOURCE_REF.
## Lighthouse's own rust-toolchain.toml controls the Rust version once the
## source is checked out; this base image only needs cargo to bootstrap.

FROM rust:1.92.0-bookworm

ARG cl_source_repo=sigp/lighthouse
ARG cl_source_ref=stable

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/lighthouse

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        clang \
        cmake \
        curl \
        git \
        jq \
        libclang-dev \
        make \
        pkg-config \
        protobuf-compiler \
        python3 \
    && rm -rf /var/lib/apt/lists/*

RUN cargo install cargo-nextest@0.9.85 --locked

RUN mkdir -p ${OUT_DIR}/junit /work

COPY entrypoint.sh /usr/local/bin/lighthouse-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/lighthouse-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/lighthouse-specs-entrypoint.sh"]
