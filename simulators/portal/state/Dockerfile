FROM rust:1.75.0 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin state
WORKDIR /state

RUN apt-get update && apt-get install clang -y

# copy over manifests and source to build image
COPY Cargo.toml ./Cargo.toml
COPY src ./src
RUN apt-get update && apt-get install clang -y

# build for release
RUN cargo build --release

# final base
FROM ubuntu:22.04

RUN apt update && apt install wget -y

# copy build artifacts from build stage
COPY --from=builder /state/target/release/state .
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/state/hive/test_data.yaml ./test-data/test_data.yaml

ENV RUST_LOG=debug

ENTRYPOINT ["./state"]
