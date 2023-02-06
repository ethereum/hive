# Docker container spec for building the master branch of nimbus.

FROM debian:buster-slim AS build

ARG branch=master

RUN apt-get update \
 && apt-get install -y --fix-missing build-essential make git libpcre3-dev librocksdb-dev \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN git clone --recurse-submodules --depth 1 --branch "${branch}" \
    https://github.com/status-im/nimbus-eth1.git

WORKDIR /nimbus-eth1

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC}" V=1 update

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC}" nimbus && \
    mv build/nimbus /usr/bin/

# --------------------------------- #
# Starting new image to reduce size #
# --------------------------------- #
FROM debian:buster-slim AS deploy

RUN apt-get update \
 && apt-get install -y librocksdb-dev bash curl jq\
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=build /usr/bin/nimbus /usr/bin/nimbus
RUN usr/bin/nimbus --version > /version.txt

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD nimbus.sh /nimbus.sh
RUN chmod +x /nimbus.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8547 8551 30303 30303/udp

ENTRYPOINT ["/nimbus.sh"]
