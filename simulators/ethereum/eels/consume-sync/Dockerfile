# Runs the EELS (execution-specs) consume sync simulator from the image published by
# ethereum/execution-specs. The image contains the simulator code and the
# blockchain_tests_sync fixtures it consumes.
# Use tag to select the tests; see ../README.md for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/consume-sync
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
