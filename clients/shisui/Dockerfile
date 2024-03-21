ARG baseimage=ghcr.io/optimism-java/shisui
ARG tag=latest

FROM $baseimage:$tag

RUN echo "latest" > /version.txt

ADD shisui.sh /shisui.sh
RUN chmod +x /shisui.sh

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9009/udp

ENTRYPOINT ["/shisui.sh"]
