# Runs the EELS (execution-specs) execute-blobs simulator from the image
# published by ethereum/execution-specs. The tests run from the source tree in
# the image, so it carries no fixtures. Use tag to select the tests; see ../README.md
# for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/execute-blobs
ARG tag=latest

FROM ${image}:${tag}

## Fork to execute the tests for. Empty uses the image's default, the fork its
## release was filled for, e.g. Amsterdam for tests@ releases.
ARG fork=""
ENV FORK=${fork}
