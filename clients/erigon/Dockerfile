## Build Erigon Via Pre-Built Image:
ARG baseimage=thorax/erigon
ARG tag=devel
FROM $baseimage:$tag

# The upstream erigon container uses a non-root user, but we need
# to install additional commands, so switch back to root.
USER root

RUN apk add --no-cache bash curl jq

# Create version.txt
RUN erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

# Add genesis mapper script, startup script, and enode URL retriever script
COPY genesis.json /genesis.json
COPY mapper.jq /mapper.jq
COPY erigon.sh /erigon.sh
COPY enode.sh /hive-bin/enode.sh

# Set execute permissions for scripts
RUN chmod +x /erigon.sh /hive-bin/enode.sh

# Expose networking ports
EXPOSE 8545 8546 8551 30303 30303/udp

ENTRYPOINT ["/erigon.sh"]
