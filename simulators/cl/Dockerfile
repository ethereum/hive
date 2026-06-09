FROM rust:1.92.0-bookworm AS builder

WORKDIR /workspace

COPY ./hivesim-rs ./hivesim-rs
COPY ./simulators/cl ./simulators/cl

RUN cd ./simulators/cl \
    && cargo build --release --locked \
    && mv target/release/cl-sim /cl-sim \
    && cargo clean \
    && rm -rf /usr/local/cargo/registry /usr/local/cargo/git

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Hive only forwards a fixed set of HIVE_* env vars to simulator
# containers (see internal/libhive/run.go: HIVE_SIMULATOR,
# HIVE_PARALLELISM, HIVE_LOGLEVEL, HIVE_TEST_PATTERN, HIVE_RANDOM_SEED).
# Other env vars set on the runner do NOT reach the simulator process.
# We carry our per-run config in via docker --build-arg (passed by
# `hive --sim.buildarg`) and bake them as ENVs so the simulator's
# env::var lookups see them at runtime.
ARG CL_SPECS_SOURCE_REPO=""
ARG CL_SPECS_SOURCE_REF=""
ARG CL_SPECS_TESTS_REF=""
ARG CL_SPECS_SCOPE=""
ARG CL_SPECS_NETWORK="unknown"
ENV CL_SPECS_SOURCE_REPO=${CL_SPECS_SOURCE_REPO} \
    CL_SPECS_SOURCE_REF=${CL_SPECS_SOURCE_REF} \
    CL_SPECS_TESTS_REF=${CL_SPECS_TESTS_REF} \
    CL_SPECS_SCOPE=${CL_SPECS_SCOPE} \
    CL_SPECS_NETWORK=${CL_SPECS_NETWORK}

COPY --from=builder /cl-sim /usr/bin/cl-sim

ENTRYPOINT ["/usr/bin/cl-sim"]
