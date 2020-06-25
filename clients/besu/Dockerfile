# Hyperledger publihes regular developer 'head' images, just use that one
FROM hyperledger/besu:develop

USER root
# Inject the startup script
ADD besu.sh /opt/besu/bin/besu-hive.sh
RUN chmod +x /opt/besu/bin/besu-hive.sh

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

# The enode retriever script requires curl
RUN apt-get update && \
    apt-get install -y jq curl

ADD genesis.json /genesis.json

ARG branch=master
RUN echo "{\
  \"repo\":\"https://github.com/hyperledger/besu\",\
  \"branch\":\"${branch:-detached}\", \
  \"commit\":\"${VCS_REF:-xxxxxx}\"\
}" > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 8547 30303 30303/udp

ENTRYPOINT ["/opt/besu/bin/besu-hive.sh"]

#EOF
