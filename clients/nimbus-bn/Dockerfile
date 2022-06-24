# Docker container spec for building the unstable branch of nimbus.

FROM debian:buster-slim AS build

RUN apt-get update \
 && apt-get install -y --fix-missing build-essential make git libpcre3-dev librocksdb-dev curl \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV NPROC=2

RUN git clone --depth 1 --branch unstable https://github.com/status-im/nimbus-eth2.git \
 && cd nimbus-eth2 \
 && make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC} -d:SECONDS_PER_SLOT=3" V=1 update

RUN cd nimbus-eth2 && \
    make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC} -d:SECONDS_PER_SLOT=3" nimbus_beacon_node && \
    mv build/nimbus_beacon_node /usr/bin/

# --------------------------------- #
# Starting new image to reduce size #
# --------------------------------- #

FROM debian:buster-slim AS deploy

RUN apt-get update \
 && apt-get install -y librocksdb-dev bash curl jq\
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=build /usr/bin/nimbus_beacon_node /usr/bin/nimbus_beacon_node

RUN usr/bin/nimbus_beacon_node --version > /version.txt

ADD nimbus_bn.sh /nimbus_bn.sh
RUN chmod +x /nimbus_bn.sh

# TODO: output accurate client version
RUN usr/bin/nimbus_beacon_node --version > /version.txt

EXPOSE 9000 9000/udp 4000 4000/udp

ENTRYPOINT ["/nimbus_bn.sh"]
