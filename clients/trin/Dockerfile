ARG branch=latest

FROM portalnetwork/trin:$branch

ADD trin.sh /trin.sh
RUN chmod +x /trin.sh

ADD trin_version.sh /trin_version.sh
RUN chmod +x /trin_version.sh

RUN /trin_version.sh > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9009/udp

ENTRYPOINT ["/trin.sh"]
