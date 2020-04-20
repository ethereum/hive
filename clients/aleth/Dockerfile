# Multistage Dockerfile for the Aleth Ethereum node.
# Build stage

# This can be run separately, as a standalone image, 
# to obtain a cache and save rebuilding all the time (during development)
#
# docker build --target builder -t aleth_base
#
ARG branch=master
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

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

RUN apk add --no-cache jq bc bash curl


# Copy buildinfo into version.json, and also remove the 'bool' field is_prerelease

# remove softlink
RUN cat /usr/share/aleth/buildinfo.json |jq "del(.is_prerelease)"  > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8547 30303 30303/udp

ENTRYPOINT ["/aleth.sh"]
