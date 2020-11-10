ARG branch=develop
FROM hyperledger/besu:$branch

USER root
RUN apt-get update && apt-get install -y curl binutils
RUN curl -L -o /usr/local/bin/jq 'https://github.com/stedolan/jq/releases/download/jq-1.6/jq-linux64' && \
    echo 'af986793a515d500ab2d35f8d2aecd656e764504b789b66d7e1a0b727a124c44  /usr/local/bin/jq' | sha256sum -c && \
    chmod +x /usr/local/bin/jq

# Add the AOT compiler script and run it.
ADD aot-compile.sh /aot-compile.sh
RUN /aot-compile.sh

# Inject the startup script.
ADD besu.sh /opt/besu/bin/besu-hive.sh
RUN chmod +x /opt/besu/bin/besu-hive.sh
ADD mapper.jq /mapper.jq
ADD genesis.json /genesis.json

# Inject the enode URL retriever script.
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
