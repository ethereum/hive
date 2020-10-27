ARG branch=develop
FROM hyperledger/besu:$branch

USER root

RUN apt-get update && apt-get install -y jq curl

# Inject the startup script
ADD besu.sh /opt/besu/bin/besu-hive.sh
RUN chmod +x /opt/besu/bin/besu-hive.sh
ADD mapper.jq /mapper.jq
ADD genesis.json /genesis.json

# Inject the enode id retriever script
ADD enode.sh /enode.sh
RUN chmod +x /enode.sh

RUN echo "{\
  \"repo\":\"https://github.com/hyperledger/besu\",\
  \"branch\":\"${branch:-detached}\", \
  \"commit\":\"${VCS_REF:-xxxxxx}\"\
}" > /version.json

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 8546 30303 30303/udp

ENTRYPOINT ["/opt/besu/bin/besu-hive.sh"]
