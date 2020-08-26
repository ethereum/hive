# Docker container spec for building the master branch of parity.

ARG branch=latest
FROM openethereum/openethereum:$branch

USER root
RUN apk add --no-cache bash bc jq curl
ADD openethereum.sh /openethereum.sh
RUN chmod +x /openethereum.sh
ADD chain.json /chain.json
RUN chmod 777 /chain.json

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh 
RUN chmod 777 /enode.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

ADD genesis.json /genesis.json
RUN chmod 777 /genesis.json

ENTRYPOINT ["/openethereum.sh"]
