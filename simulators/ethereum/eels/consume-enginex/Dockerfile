# Runs the EELS (execution-specs) consume enginex simulator from the image published by
# ethereum/execution-specs. The image contains the simulator code and the
# blockchain_tests_engine_x fixtures it consumes, pre-alloc groups included.
# Use tag to select the tests; see ../README.md for build arguments.
ARG image=ghcr.io/ethereum/execution-specs/hive/consume-enginex
ARG tag=latest

FROM ${image}:${tag}

# TODO: remove when fixed on nimbus-el side.
ARG disable_strict_exception_matching=nimbus-el
ENV DISABLE_STRICT_EXCEPTION_MATCHING=${disable_strict_exception_matching}
