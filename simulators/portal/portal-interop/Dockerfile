FROM rust:1.71.1 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin portal-interop
WORKDIR /portal-interop

# copy over manifests and source to build image
COPY Cargo.toml ./Cargo.toml
COPY src ./src

# build for release
RUN cargo build --release

# final base
FROM ubuntu:22.04

RUN apt update && apt install wget -y

# copy build artifacts from build stage
COPY --from=builder /portal-interop/target/release/portal-interop .
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/history/hive/test_data_collection_of_forks_blocks.yaml ./test-data/test_data_collection_of_forks_blocks.yaml

ENV RUST_LOG=debug

ENTRYPOINT ["./portal-interop"]
