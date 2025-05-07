FROM rust:latest AS builder

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
FROM ubuntu:24.04

# declare build args for second stage
ARG PORTAL_CONSENSUS_URL
ARG PORTAL_CONSENSUS_AUTH

# set the environment variables
ENV PORTAL_CONSENSUS_URL=$PORTAL_CONSENSUS_URL
ENV PORTAL_CONSENSUS_AUTH=$PORTAL_CONSENSUS_AUTH

RUN apt-get update && apt-get install -y wget git ca-certificates

# copy build artifacts from build stage
COPY --from=builder /portal/target/release/portal .

# clone portal-spec-tests repo. The add command is used to reset docker's cache so new data is pulled
ADD "https://api.github.com/repos/ethereum/portal-spec-tests/commits?per_page=1" latest_commit
RUN git clone https://github.com/ethereum/portal-spec-tests.git ./portal-spec-tests

ENV RUST_LOG=debug,hyper_util=info,hyper=info,reqwest=info

ENTRYPOINT ["./portal"]
