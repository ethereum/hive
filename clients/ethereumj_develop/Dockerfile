# Docker container spec for building the master branch of EthereumJ.
#
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.
FROM anapsix/alpine-java:8_jdk_unlimited

# Build Harmony on the fly and delete all build tools afterwards
RUN \
  apk add --update git jq                                                               && \
  git clone --depth 1 -b develop https://github.com/ether-camp/ethereum-harmony.git     && \
  (cd ethereum-harmony && git checkout develop  && git pull                             && \
  echo "{}"                                                                  \
  | jq ".+ {\"repo\":\"$(git config --get remote.origin.url)\"}"             \
  | jq ".+ {\"branch\":\"$(git rev-parse --abbrev-ref HEAD)\"}"              \
  | jq ".+ {\"commit\":\"$(git rev-parse HEAD)\"}"                           \
	> /version.json)                                                                    && \
  (cd /ethereum-harmony && ./gradlew distTar -PrpcOnly -x test)                         && \
  (cd /ethereum-harmony/build/distributions/ && tar -C / -xf harmony.ether.camp.tar)    && \
  apk del git


# Make sure bash and jq is available for easier wrapper implementation
RUN apk add --update bash jq

# Inject the startup script
ADD harmony.sh /harmony.sh
RUN chmod +x /harmony.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/harmony.sh"]

#EOF
