ARG branch=nightly
FROM ethereum/aleth:$branch

# The aleth base docker image uses 'aleth' user. Switch to root so our additions work.
USER root

RUN apk add --no-cache jq bc bash curl

# Add the startup script.
ADD aleth.sh /aleth.sh
RUN chmod +x /aleth.sh
ADD mapper.jq /mapper.jq
ADD genesis.json /genesis.json

# Add the enode URL retriever script.
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

# Copy buildinfo into version.json, and also remove the 'bool' field is_prerelease.
RUN jq -r '.version' /usr/share/aleth/buildinfo.json > /version.txt

# Export the usual networking ports to allow outside access to the node.
EXPOSE 8545 8547 30303 30303/udp

ENTRYPOINT ["/aleth.sh"]
