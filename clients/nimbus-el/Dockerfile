ARG tag=master
ARG baseimage=statusim/nimbus-eth1

FROM $baseimage:$tag AS source

FROM debian:testing-slim AS deploy

SHELL ["/bin/bash", "-c"]

RUN apt-get clean \
    && apt-get update \
    && apt-get install -y build-essential librocksdb-dev jq curl \
    && apt -y upgrade

RUN ldd --version

COPY --from=source /home/user/nimbus-eth1/build/nimbus_execution_client /usr/bin/nimbus_execution_client

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
