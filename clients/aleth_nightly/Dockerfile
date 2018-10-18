# Multistage Dockerfile for the Aleth Ethereum node.
# Build stage

# This can be run separately, as a standalone image, 
# to obtain a cache and save rebuilding all the time (during development)
#
# docker build --target builder -t aleth_base
#

FROM ethereum/aleth:nightly

# Genesis template
ADD config.json /config.json

# Inject the startup script
ADD aleth.sh /aleth.sh
RUN chmod +x /aleth.sh

RUN apk add --no-cache jq bc bash

# Copy buildinfo into version.json, and also remove the 'bool' field is_prerelease

# remove softlink
RUN cat /usr/share/aleth/buildinfo.json |jq "del(.is_prerelease)"  > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303 30303/udp

ENTRYPOINT ["/aleth.sh"]
