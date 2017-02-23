# Docker container spec for building the master branch of go-ethereum.
#
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.
FROM alpine:latest


# Build go-ethereum on the fly and delete all build tools afterwards
RUN \
  apk add --update bash jq go git make gcc musl-dev              \ 
  	  ca-certificates linux-headers                           && \
  git clone --depth 1 https://github.com/ethereum/go-ethereum && \
  (cd go-ethereum && make geth)                               && \
  cp go-ethereum/build/bin/geth /geth                         && \
  apk del go git make gcc musl-dev linux-headers              && \
  rm -rf /go-ethereum && rm -rf /var/cache/apk/*

# Inject the startup script
ADD geth.sh /geth.sh
RUN chmod +x /geth.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303

ENTRYPOINT ["/geth.sh"]
