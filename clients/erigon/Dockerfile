# Docker container spec for building the master branch of erigon.
#   
# The build process it potentially longer running but every effort was made to
# produce a very minimalistic container that can be reused many times without
# needing to constantly rebuild.

ARG branch=devel
FROM thorax/erigon:$branch

RUN apt-get -y update && apt-get install -y bash curl jq

ADD genesis.json /genesis.json
ADD mapper.jq /mapper.jq

# Inject the startup script
ADD erigon.sh /erigon.sh
RUN chmod +x /erigon.sh

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

# Add dummy /version.txt
RUN ./erigon --version | sed -e 's/erigon version \(.*\)/\1/' > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/erigon.sh"]
