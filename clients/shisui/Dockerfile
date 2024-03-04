ARG baseimage=ghcr.io/optimism-java/shisui
ARG tag=latest

FROM $baseimage:$tag

RUN echo "latest" > /version.txt

# Export the usual networking ports to allow outside access to the node
EXPOSE 8545 9009/udp

ENTRYPOINT ["app"]
