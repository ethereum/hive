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

# The /version.json is a softlink, which hive won't resolve when trying to read it.
# We copy it into a regular file, and also remove the 'bool' field is_prerelease


RUN cat /version.json | jq "del(.is_prerelease)" > /x.json && \
	rm /version.json && \
    mv /x.json /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 30303

ENTRYPOINT ["/aleth.sh"]
