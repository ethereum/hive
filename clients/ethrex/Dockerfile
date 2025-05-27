# This is the image we build on a push to main
# We do have a problem, if we're testing locally it doesn't show changes
# Removing this from here now, but the upstream version should use the ghcr.io version
# ARG baseimage=ghcr.io/lambdaclass/ethrex 
ARG baseimage=ethrex # We don

ARG tag=latest

FROM $baseimage:$tag AS builder

# Install script tools.
RUN apt-get update -y
RUN apt-get install -y bash curl jq

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD ethrex.sh /ethrex.sh
RUN chmod +x /ethrex.sh

# Add the enode URL retriever script.
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Create version.txt
RUN ethrex --version | sed -e 's/ethrex \(.*\)/\1/' > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/ethrex.sh"]
