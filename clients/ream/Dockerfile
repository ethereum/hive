ARG baseimage=ghcr.io/reamlabs/ream
ARG tag=latest-devnet3

FROM $baseimage:$tag

COPY validators.yaml /assets/lean/validators.yaml
ADD ream.sh /ream.sh

RUN chmod +x /ream.sh \
    && ream --version | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/ream.sh"]
