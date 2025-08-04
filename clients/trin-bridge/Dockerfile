ARG baseimage=portalnetwork/trin
ARG tag=latest-bridge

FROM $baseimage:$tag

ADD https://raw.githubusercontent.com/ethereum/portal-spec-tests/master/tests/mainnet/history/hive/test_data_collection_of_forks_blocks.yaml /test_data_collection_of_forks_blocks.yaml
ADD trin_bridge.sh /trin_bridge.sh
RUN chmod +x /trin_bridge.sh

RUN echo "latest" > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9009/udp

# add fake secrets for bridge activation
ENV PANDAOPS_CLIENT_ID=xxx
ENV PANDAOPS_CLIENT_SECRET=xxx

ENTRYPOINT ["/trin_bridge.sh"]
