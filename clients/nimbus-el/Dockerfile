# Docker container spec for building the master branch of nimbus.

FROM debian:testing-slim AS build

SHELL ["/bin/bash", "-c"]

RUN apt-get clean && apt update \
 && apt -y install curl build-essential git-lfs librocksdb-dev

RUN ldd --version ldd

ARG tag=master
ARG github=status-im/nimbus-eth1

RUN git clone --branch "$tag" https://github.com/$github

WORKDIR /nimbus-eth1

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC}" V=1 update

RUN NPROC=$(nproc); make -j${NPROC} NIMFLAGS="--parallelBuild:${NPROC} -d:chronicles_colors=none" nimbus_execution_client && \
    mv build/nimbus_execution_client /usr/bin/

# --------------------------------- #
# Starting new image to reduce size #
# --------------------------------- #
FROM debian:testing-slim AS deploy

RUN apt-get clean && apt update \
 && apt -y install build-essential librocksdb-dev jq curl
RUN apt update && apt -y upgrade

RUN ldd --version ldd

COPY --from=build /usr/bin/nimbus_execution_client /usr/bin/nimbus_execution_client
RUN usr/bin/nimbus_execution_client --version > /version.txt

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