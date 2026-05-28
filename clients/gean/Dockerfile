ARG devnet4_baseimage=ghcr.io/geanlabs/gean
ARG devnet4_tag=devnet4

FROM $devnet4_baseimage:$devnet4_tag

COPY validators.yaml /assets/lean/validators.yaml
RUN cp /usr/local/bin/gean /usr/local/bin/gean-devnet4
ADD gean.sh /gean.sh

RUN chmod +x /gean.sh \
    && (/usr/local/bin/gean-devnet4 --version 2>/dev/null || echo "gean-devnet4") | head -1 > /version-devnet4.txt \
    && printf "gean devnet4: %s\n" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/gean.sh"]
