# Runs the EELS (execution-specs) consume sync simulator from the image published by
# ethereum/execution-specs. The image contains the simulator code and the
# blockchain_tests_sync fixtures it consumes.
# Use tag to select the tests; see ../README.md for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/consume-sync
ARG tag=
# Deprecated: without a tag, the Dockerfile.git arguments fixtures and branch
# select the image's latest-legacy variant, which applies them during this
# build as a source build did, see ethereum/execution-specs#3866. Without
# them, the published image is used directly.
ARG fixtures=
ARG branch=
ARG legacy=${fixtures}${branch}

FROM ${image}:${tag:-latest${legacy:+-legacy}}
