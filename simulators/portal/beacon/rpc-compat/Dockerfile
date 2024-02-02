FROM rust:1.71.1 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin rpc-compat
WORKDIR /rpc-compat

RUN apt-get update && apt-get install clang -y

# copy over manifests and source to build image
COPY Cargo.toml ./Cargo.toml
COPY src ./src

# build for release
RUN cargo build --release

# final base
FROM ubuntu:22.04

RUN apt update && apt install wget -y

# copy build artifacts from build stage
COPY --from=builder /rpc-compat/target/release/rpc-compat .

ENV RUST_LOG=debug

ENTRYPOINT ["./rpc-compat"]
