# This dockerfile builds OpenEthereum for Hive.
ARG tag=latest
ARG baseimage=openethereum/openethereum

# The runner script needs a tool to convert raw private keys
# to keystore files. We use the ethkey tool from geth for this.
FROM ethereum/client-go:alltools-latest AS keytool-builder

# Now pull OpenEthereum.
FROM $baseimage:$tag
USER root

# Copy keystore tool.
COPY --from=keytool-builder /usr/local/bin/ethkey /ethkey

# Install additional tools.
RUN apk add --no-cache bash bc jq curl

ADD openethereum.sh /openethereum.sh
RUN chmod +x /openethereum.sh
ADD mapper.jq /mapper.jq

# Inject the enode id retriever script
RUN mkdir /hive-bin
ADD enode.sh /hive-bin/enode.sh
RUN chmod +x /hive-bin/enode.sh

# Add dummy /version.json
RUN ./openethereum --version > /raw_version.txt
RUN head -n 2 /raw_version.txt | tail -1 > /version.txt
RUN sed -i 's/\<version\>//g' /version.txt
RUN sed -i 's/^[[:space:]]*//' /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

# Add default genesis.json
ADD genesis.json /genesis.json
RUN chmod 777 /genesis.json

ENTRYPOINT ["/openethereum.sh"]
