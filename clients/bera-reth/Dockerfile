ARG baseimage=ghcr.io/berachain/bera-reth
ARG tag=nightly
# Replacement image and tag to use local debug image built using `make docker-build-debug` from bera-reth repo
#ARG baseimage=bera-reth
#ARG tag=debug

FROM $baseimage:$tag as builder

# Install script tools.
RUN apt-get update -y
RUN apt-get install -y bash curl jq

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD bera-reth.sh /bera-reth.sh
RUN chmod +x /bera-reth.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Create version.txt
RUN /usr/local/bin/bera-reth --version | head -2 | awk 'NR==1 {gsub("bera-reth ", ""); version=$0} NR==2 {gsub("Commit SHA: ", ""); sha=substr($0, 1, 8)} END {print version"+"sha}' > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/bera-reth.sh"]