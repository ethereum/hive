FROM rust:latest AS builder

RUN USER=root cargo new --bin lean-sim
WORKDIR /lean-sim

COPY Cargo.toml ./Cargo.toml
COPY Cargo.lock ./Cargo.lock

RUN echo "fn main() {}" > ./src/main.rs
RUN cargo build --release --locked

COPY src ./src
RUN touch -a -m ./src/main.rs
RUN cargo build --release --locked

FROM ubuntu:24.04

RUN apt-get update && apt-get install -y ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /lean-sim/target/release/lean-sim /usr/bin/lean-sim

ENTRYPOINT ["/usr/bin/lean-sim"]
