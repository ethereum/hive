ARG branch=nightly
FROM ethereum/aleth:$branch

# Genesis template
ADD config.json /config.json

# Inject the startup script
ADD aleth.sh /aleth.sh
# The aleth base docker image uses 'aleth' user. The ADD operation above,
# however, adds the file with root ownership. So in order to chmod, we need
# to switch to root.
USER root
RUN chmod +x /aleth.sh

# Inject the enode retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

RUN apk add --no-cache jq bc bash curl

# Copy buildinfo into version.json, and also remove the 'bool' field is_prerelease
RUN jq "del(.is_prerelease)" /usr/share/aleth/buildinfo.json > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

ENTRYPOINT ["/aleth.sh"]
