# Hyperledger publihes regular developer 'head' images, just use that one
FROM hyperledger/besu:develop

# Inject the startup script
ADD besu.sh /opt/besu/bin/besu-hive.sh
RUN chmod +x /opt/besu/bin/besu-hive.sh

# Inject the enode id retriever script
ADD enode.sh /opt/besu/bin/enode.sh
RUN chmod +x /opt/besu/bin/enode.sh

RUN apt-get update && \
    apt-get install -y jq

ADD genesis.json /etc/besu/genesis.json

ARG branch=master
RUN echo "{\
  \"repo\":\"https://github.com/hyperledger/besu\",\
  \"branch\":\"${branch:-detachted}\", \
  \"commit\":\"${VCS_REF:-xxxxxx}\"\
}" > /version.json    

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/opt/besu/bin/besu-hive.sh"]

#EOF
