## Build Besu Via Pre-Built Image:
ARG baseimage=hyperledger/besu
ARG tag=develop
FROM $baseimage:$tag

USER root

# Update, install curl, download and install jq, and clean up in one step
RUN apt-get update && apt-get install -y curl jq && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# Create version.txt
RUN /opt/besu/bin/besu --version > /version.txt

# Add genesis mapper script, startup script, and enode URL retriever script
COPY genesis.json /genesis.json
COPY mapper.jq /mapper.jq
COPY besu.sh /opt/besu/bin/besu-hive.sh
COPY enode.sh /hive-bin/enode.sh
COPY trusted_setup.txt /trusted_setup.txt

# Set execute permissions for scripts
RUN chmod +x /opt/besu/bin/besu-hive.sh /hive-bin/enode.sh

# Expose networking ports (incl 5005 for JVM debugging)
EXPOSE 8545 8546 8551 30303 30303/udp 5005

ENTRYPOINT ["/opt/besu/bin/besu-hive.sh"]
