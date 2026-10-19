# Runs the EELS (execution-specs) execute-blobs simulator from the image
# published by ethereum/execution-specs. The tests run from the source tree in
# the image, so it carries no fixtures. Use tag to select the tests; see ../README.md
# for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/execute-blobs
ARG tag=

FROM ${image}:${tag:-latest}

# Deprecated: without a tag, the Dockerfile.git arguments fixtures and branch
# still select a source build of the fixtures and framework, see
# ethereum/execution-specs#3866. Pass tag= to use a published image.
ARG tag
ARG fixtures
ARG branch
COPY Dockerfile fixtures.tar.gz* /staged/
RUN eels-legacy-build

## Fork to execute the tests for. Empty uses the image's default, the fork its
## release was filled for.
ARG fork=""
ENV FORK=${fork}
