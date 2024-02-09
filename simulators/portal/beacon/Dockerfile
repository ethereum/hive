FROM rust:1.75.0 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin beacon
WORKDIR /beacon

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
COPY --from=builder /beacon/target/release/beacon .

ENV RUST_LOG=debug

ENTRYPOINT ["./beacon"]
