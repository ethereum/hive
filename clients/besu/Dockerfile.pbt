### Build Besu with its PBT library (besu-stateless) from git.
#
# The PBT branch of Besu depends on a besu-stateless SNAPSHOT that is not
# published to any public Maven repository. This image builds the library from
# its branch, publishes it to the builder's local Maven repository, which
# Besu's build already resolves from (mavenLocal), and then builds Besu.

## Builder stage
FROM ubuntu:24.04 AS builder

ARG github=matkt/besu
ARG tag=glamsterdam-devnet-8-pbt
ARG stateless_github=besu-eth/besu-stateless
ARG stateless_tag=feat/partitioned-binary-trie

RUN apt-get update && apt-get install -y git libsodium-dev libnss3-dev \
    && apt-get install --no-install-recommends -q --assume-yes ca-certificates-java \
    && apt-get install --no-install-recommends -q --assume-yes openjdk-25-jdk-headless libjemalloc-dev

# 1. The PBT library, into ~/.m2.
RUN echo "Cloning: $stateless_github - $stateless_tag" \
    && git clone --depth 1 --branch $stateless_tag https://github.com/$stateless_github stateless \
    && cd stateless && ./gradlew --no-daemon publishToMavenLocal -x test

# 2. Besu, which must ask for the version the library just published.
RUN echo "Cloning: $github - $tag" \
    && git clone --depth 1 --branch $tag https://github.com/$github besu \
    && lib=$(sed -n 's/^version=//p' stateless/gradle.properties) \
    && want=$(sed -n 's/^besuStatelessVersion=//p' besu/gradle.properties) \
    && { [ "$lib" = "$want" ] || { echo "besu wants besu-stateless $want, the library branch is $lib" >&2; exit 1; }; } \
    && cd besu && ./gradlew --no-daemon installDist

## Final stage: identical to Dockerfile.git
FROM ubuntu:24.04

COPY --from=builder /besu/build/install/besu /opt/besu

RUN apt-get update && apt-get install -y curl jq libsodium23 libnss3-dev \
    && apt-get install --no-install-recommends -q --assume-yes ca-certificates-java \
    && apt-get install --no-install-recommends -q --assume-yes openjdk-25-jre-headless libjemalloc-dev \
    && apt-get clean && rm -rf /var/lib/apt/lists/*

RUN /opt/besu/bin/besu --version > /version.txt

COPY genesis.json /genesis.json
COPY mapper.jq /mapper.jq
COPY besu.sh /opt/besu/bin/besu-hive.sh
COPY enode.sh /hive-bin/enode.sh
RUN chmod +x /opt/besu/bin/besu-hive.sh /hive-bin/enode.sh

EXPOSE 8545 8546 8551 30303 30303/udp 5005
ENTRYPOINT ["/opt/besu/bin/besu-hive.sh"]
