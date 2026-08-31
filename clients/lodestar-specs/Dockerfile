## Lodestar consensus-spec-tests runner. Built from source via pnpm at
## container startup so HIVE_CL_SOURCE_REF can be any tag/branch/SHA.

FROM node:24-bookworm

ARG cl_source_repo=ChainSafe/lodestar
ARG cl_source_ref=unstable

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/lodestar

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        jq \
        make \
        python3 \
    && rm -rf /var/lib/apt/lists/*

RUN corepack enable

RUN mkdir -p ${OUT_DIR}/junit /work

COPY entrypoint.sh /usr/local/bin/lodestar-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/lodestar-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/lodestar-specs-entrypoint.sh"]
