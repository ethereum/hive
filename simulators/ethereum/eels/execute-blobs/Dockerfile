# Runs the EELS (execution-specs) execute-blobs simulator from the image
# published by ethereum/execution-specs. The tests run from the source tree in
# the image, so it carries no fixtures. Use tag to select the tests; see ../README.md
# for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/execute-blobs
ARG tag=
# Deprecated: without a tag, the Dockerfile.git arguments fixtures and branch
# select the image's latest-legacy variant, which applies them during this
# build as a source build did, see ethereum/execution-specs#3866. Without
# them, the published image is used directly.
ARG fixtures=
ARG branch=
ARG legacy=${fixtures}${branch}

FROM ${image}:${tag:-latest${legacy:+-legacy}}

## Fork to execute the tests for. Empty uses the image's default, the fork its
## release was filled for.
ARG fork=""
ENV FORK=${fork}
