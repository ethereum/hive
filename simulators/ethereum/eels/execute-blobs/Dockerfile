# Runs the EELS (execution-specs) execute-blobs simulator from the image
# published by ethereum/execution-specs. The tests run from the source tree in
# the image, so it carries no fixtures. The tag is the only build argument that
# selects what runs; see README.md in this directory for the tag scheme.
ARG image=ghcr.io/ethereum/execution-specs/hive/execute-blobs
ARG tag=latest

FROM ${image}:${tag}

## Default fork
ARG fork=Osaka
ENV FORK=${fork}
