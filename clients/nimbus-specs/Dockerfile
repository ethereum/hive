## Nimbus consensus-spec-tests runner. Builds nimbus-eth2's vendored nim
## compiler and the `consensus_spec_tests_<preset>` test binary at container
## startup, then runs it with --xml output.

FROM debian:bookworm

ARG cl_source_repo=status-im/nimbus-eth2
ARG cl_source_ref=unstable

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/nimbus-eth2

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        build-essential \
        ca-certificates \
        curl \
        git \
        git-lfs \
        jq \
        libpcre3-dev \
        make \
        python3 \
    && rm -rf /var/lib/apt/lists/* \
    && git lfs install --system

RUN mkdir -p ${OUT_DIR}/junit /work

COPY entrypoint.sh /usr/local/bin/nimbus-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/nimbus-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/nimbus-specs-entrypoint.sh"]
