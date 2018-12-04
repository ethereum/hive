# Docker container spec for running a local go-ethereum binary.
#
# This docker image spec is meant to be used during client development, meaning
# that the build process is designed for short-livedness and frequent rebuilds.
# As such build times are short, possibly resulting in higher disk usage.
FROM ubuntu:16.04

# Since Go binaries are cross Linux portable, we don't need many dependencies aside
# from the stuff that's already included in the base Ubuntu image. The Geth binary
# itself will be mounted into the container during startup.
RUN apt-get update && apt-get install -y jq --no-install-recommends

# Inject the startup script
ADD geth.sh /geth.sh
RUN chmod +x /geth.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303 30303/udp

ADD version.json /version.json
ENTRYPOINT ["/geth.sh"]
