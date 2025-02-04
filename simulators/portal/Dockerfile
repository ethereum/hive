FROM rust:1.81.0 AS builder

# create a new empty shell project
RUN USER=root cargo new --bin portal
WORKDIR /portal

RUN apt-get update && apt-get install clang -y

# copy over manifests and source to build image
COPY Cargo.toml ./Cargo.toml
COPY Cargo.lock ./Cargo.lock

# create fake main.rs and build dependencies
RUN echo "fn main() {}" > ./src/main.rs

RUN cargo build --release

# copy real src over and make newest modified file
COPY src ./src

RUN touch -a -m ./src/main.rs

# build for release
RUN cargo build --release

# final base
FROM ubuntu:22.04

# declare build args for second stage
ARG PORTAL_CONSENSUS_URL
ARG PORTAL_CONSENSUS_AUTH

# set the environment variables
ENV PORTAL_CONSENSUS_URL=$PORTAL_CONSENSUS_URL
ENV PORTAL_CONSENSUS_AUTH=$PORTAL_CONSENSUS_AUTH

RUN apt-get update && apt-get install -y wget

# copy build artifacts from build stage
COPY --from=builder /portal/target/release/portal .
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/history/hive/test_data_collection_of_forks_blocks.yaml ./test-data/test_data_collection_of_forks_blocks.yaml
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/state/hive/test_data.yaml ./test-data/state_test_data.yaml
ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/beacon_chain/hive/test_data.yaml ./test-data/beacon_test_data.yaml

ENV RUST_LOG=debug,hyper_util=info,hyper=info,reqwest=info

ENTRYPOINT ["./portal"]
