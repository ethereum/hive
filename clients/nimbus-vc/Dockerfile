# Docker container spec for building the unstable branch of nimbus.

FROM debian:buster-slim AS build

ARG branch=unstable

RUN apt-get update \
 && apt-get install -y --fix-missing build-essential make git libpcre3-dev librocksdb-dev curl \
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

RUN git clone --recurse-submodules --depth 1 --branch "${branch}" \
    https://github.com/status-im/nimbus-eth2.git

WORKDIR /nimbus-eth2

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC}" V=1 update

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC}" nimbus_validator_client && \
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

# Add the startup script.
ADD nimbus_vc.sh /nimbus_vc.sh
RUN chmod +x /nimbus_vc.sh

ADD nimbus_version.sh /nimbus_version.sh
RUN chmod +x /nimbus_version.sh

RUN /nimbus_version.sh > /version.txt

ENTRYPOINT ["/nimbus_vc.sh"]
