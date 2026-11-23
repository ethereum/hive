ARG tag=master
ARG baseimage=statusim/nimbus-eth1

FROM $baseimage:$tag AS source

FROM debian:testing-slim AS deploy

ARG parallel_state_root=""
ENV HIVE_PARALLEL_STATE_ROOT=${parallel_state_root}
ARG eager_state_root=""
ENV HIVE_EAGER_STATE_ROOT=${eager_state_root}
ARG sto_static_lookup_disabled=""
ENV HIVE_STO_STATIC_LOOKUP_DISABLED=${sto_static_lookup_disabled}

SHELL ["/bin/bash", "-c"]

RUN apt-get clean \
    && apt-get update \
    && apt-get install -y build-essential jq curl \
    && apt -y upgrade

RUN ldd --version

COPY --from=source /home/user/nimbus-eth1/build/nimbus /usr/bin/nimbus

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
