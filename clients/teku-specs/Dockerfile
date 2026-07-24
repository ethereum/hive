## Teku consensus-spec-tests runner. Runs gradle's reference-test task at
## container startup and harvests gradle's per-test-class JUnit XMLs.

FROM eclipse-temurin:25-jdk-noble

ARG cl_source_repo=Consensys/teku
ARG cl_source_ref=master

ENV HIVE_CL_SOURCE_REPO_DEFAULT=${cl_source_repo} \
    HIVE_CL_SOURCE_REF_DEFAULT=${cl_source_ref} \
    OUT_DIR=/out \
    SRC_DIR=/work/teku

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        git \
        jq \
        python3 \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p ${OUT_DIR}/junit /work

COPY entrypoint.sh /usr/local/bin/teku-specs-entrypoint.sh
COPY serve.sh      /usr/local/bin/cl-specs-serve.sh
RUN chmod +x /usr/local/bin/teku-specs-entrypoint.sh /usr/local/bin/cl-specs-serve.sh

EXPOSE 5151

ENTRYPOINT ["/usr/local/bin/teku-specs-entrypoint.sh"]
