FROM rust:1.71.1 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin trin-bridge
WORKDIR /trin-bridge

RUN apt-get update && apt-get install clang -y

# copy over manifests and source to build image
COPY Cargo.toml ./Cargo.toml
COPY src ./src

# build for release
RUN cargo build --release

# final base
FROM ubuntu:22.04

RUN apt update && apt install wget -y

# Use these for amd-based devices
RUN wget http://nz2.archive.ubuntu.com/ubuntu/pool/main/o/openssl/libssl1.1_1.1.0g-2ubuntu4_amd64.deb
RUN dpkg -i libssl1.1_1.1.0g-2ubuntu4_amd64.deb
# Use these for arm-based devices
#RUN wget http://ports.ubuntu.com/pool/main/o/openssl/libssl1.1_1.1.1f-1ubuntu2_arm64.deb
#RUN dpkg -i libssl1.1_1.1.1f-1ubuntu2_arm64.deb

# copy build artifacts from build stage
COPY --from=builder /trin-bridge/target/release/trin-bridge .
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/history/hive/test_data_collection_of_forks_blocks.yaml ./test-data/test_data_collection_of_forks_blocks.yaml

RUN apt-get update && apt-get install libcurl4 -y

ENV RUST_LOG=debug

# Fake secrets for Trin bridge activation, not actually used in the used `BridgeMode::Test` mode
ENV PANDAOPS_CLIENT_ID=xxx
ENV PANDAOPS_CLIENT_SECRET=xxx

ENTRYPOINT ["./trin-bridge"]
