## Prysm consensus-spec-tests runner. Uses bazelisk to drive `bazel test`
## against the prysm spectest packages and harvests per-target test.xml
## from bazel-testlogs.

FROM debian:bookworm

ARG cl_source_repo=OffchainLabs/prysm
ARG cl_source_ref=develop
ARG bazelisk_version=v1.22.0

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/prysm

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        jq \
        python3 \
        unzip \
        wget \
        zip \
    && rm -rf /var/lib/apt/lists/*

RUN ARCH=$(dpkg --print-architecture) \
    && case "${ARCH}" in \
        amd64) BAZEL_BIN=bazelisk-linux-amd64 ;; \
        arm64) BAZEL_BIN=bazelisk-linux-arm64 ;; \
        *) echo "unsupported arch ${ARCH}"; exit 1 ;; \
    esac \
    && curl -fsSL -o /usr/local/bin/bazel \
        "https://github.com/bazelbuild/bazelisk/releases/download/${bazelisk_version}/${BAZEL_BIN}" \
    && chmod +x /usr/local/bin/bazel

RUN mkdir -p ${OUT_DIR}/junit /work

COPY entrypoint.sh /usr/local/bin/prysm-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/prysm-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/prysm-specs-entrypoint.sh"]
