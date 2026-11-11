## Grandine consensus-spec-tests runner. Builds grandine workspace from
## source via cargo-nextest at container startup.

FROM rust:1.92.0-bookworm

ARG cl_source_repo=grandinetech/grandine
ARG cl_source_ref=develop

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/grandine

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

COPY entrypoint.sh /usr/local/bin/grandine-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/grandine-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/grandine-specs-entrypoint.sh"]
