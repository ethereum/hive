ARG baseimage=ethereum_rust
ARG tag=latest

FROM $baseimage:$tag as builder

# Install script tools.
RUN apt-get update -y
RUN apt-get install -y bash curl jq

# Add genesis mapper script.
ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Add the startup script.
ADD ethereum_rust.sh /ethereum_rust.sh
RUN chmod +x /ethereum_rust.sh

# Add the enode URL retriever script.
# ADD enode.sh /hive-bin/enode.sh
# RUN chmod +x /hive-bin/enode.sh

# Create version.txt
# RUN /usr/local/bin/ethereum_rust --version | sed -e 's/ethereum_rust \(.*\)/\1/' > /version.txt
RUN echo "ethereum_rust/0.1.0" > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/ethereum_rust.sh"]
