# Docker container spec for building the unstable branch of nimbus.

FROM debian:buster-slim AS build

RUN apt-get update \
 && apt-get install -y --fix-missing build-essential make git libpcre3-dev librocksdb-dev curl \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

ENV NPROC=2

RUN git clone --depth 1 --branch unstable https://github.com/status-im/nimbus-eth2.git \
 && cd nimbus-eth2 \
 && make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC} -d:SECONDS_PER_SLOT=6" V=1 update

RUN cd nimbus-eth2 && \
    make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC} -d:SECONDS_PER_SLOT=6" nimbus_validator_client && \
    mv build/nimbus_validator_client /usr/bin/

# --------------------------------- #
# Starting new image to reduce size #
# --------------------------------- #

FROM debian:buster-slim AS deploy

RUN apt-get update \
 && apt-get install -y librocksdb-dev bash curl jq\
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=build /usr/bin/nimbus_validator_client /usr/bin/nimbus_validator_client

RUN usr/bin/nimbus_validator_client --version > /version.txt

# Add the startup script.
ADD nimbus_vc.sh /nimbus_vc.sh
RUN chmod +x /nimbus_vc.sh

ENTRYPOINT ["/nimbus_vc.sh"]
