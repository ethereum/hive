# Runs the EELS (execution-specs) consume sync simulator from the image published by
# ethereum/execution-specs. The image contains the simulator code and the
# blockchain_tests_sync fixtures it consumes.
# The tag is the only build argument that selects what runs; see README.md in
# this directory for the tag scheme.
ARG image=ghcr.io/ethereum/execution-specs/hive/consume-sync
ARG tag=latest

FROM ${image}:${tag}
