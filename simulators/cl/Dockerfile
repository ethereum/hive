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

COPY --from=builder /cl-sim /usr/bin/cl-sim

ENTRYPOINT ["/usr/bin/cl-sim"]
