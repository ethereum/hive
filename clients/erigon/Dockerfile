ARG branch=devel
FROM thorax/erigon:$branch

# The upstream erigon container uses a non-root user, but we need
# to install additional commands, so switch back to root.
USER root

# Install script tools.
RUN apk add --update bash curl jq

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD erigon.sh /erigon.sh
RUN chmod +x /erigon.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Create version.txt
RUN /usr/local/bin/erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8546 8551 30303 30303/udp

ENTRYPOINT ["/erigon.sh"]
