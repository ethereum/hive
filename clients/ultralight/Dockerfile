
ARG baseimage=ghcr.io/ethereumjs/ultralight
ARG tag=latest

FROM $baseimage:$tag

COPY ultralight.sh /ultralight.sh
RUN chmod +x /ultralight.sh

RUN echo "latest" > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9000/udp

ENTRYPOINT ["/ultralight.sh"]
