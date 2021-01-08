# This dockerfile builds OpenEthereum for Hive.
ARG branch=latest

# The runner script needs a tool to convert raw private keys
# to keystore files. We use the ethkey tool from geth for this.
FROM ethereum/client-go:alltools-latest AS keytool-builder

# Now pull OpenEthereum.
FROM openethereum/openethereum:$branch
USER root

# Copy keystore tool.
COPY --from=keytool-builder /usr/local/bin/ethkey /ethkey

# Install additional tools.
RUN apk add --no-cache bash bc jq curl

ADD openethereum.sh /openethereum.sh
RUN chmod +x /openethereum.sh
ADD mapper.jq /mapper.jq

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh
RUN chmod 777 /enode.sh

# Add dummy /version.json
RUN echo "{\"repo\":\"\", \"branch\":\"\", \"commit\":\"\"}" > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

# Add default genesis.json
ADD genesis.json /genesis.json
RUN chmod 777 /genesis.json

ENTRYPOINT ["/openethereum.sh"]
