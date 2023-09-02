## Build Erigon Via Pre-Built Image:
ARG baseimage=thorax/erigon
ARG tag=devel
FROM $baseimage:$tag

# The upstream erigon container uses a non-root user, but we need
# to install additional commands, so switch back to root.
USER root

# Install script tools.
RUN apk add --no-cache bash curl jq

# Add the startup script.
ADD erigon.sh /erigon.sh
RUN chmod +x /erigon.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Create version.txt
RUN erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

# Expose networking ports
EXPOSE 8545 8546 8551 30303 30303/udp

ENTRYPOINT ["/erigon.sh"]
