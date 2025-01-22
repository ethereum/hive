ARG baseimage=portalnetwork/trin
ARG tag=latest

FROM $baseimage:$tag

ADD trin.sh /trin.sh
RUN chmod +x /trin.sh

# Create version.txt
RUN /usr/bin/trin --version | head -1 > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9009/udp

ENTRYPOINT ["/trin.sh"]
