ARG devnet3_baseimage=ghcr.io/geanlabs/gean
ARG devnet3_tag=devnet3
ARG devnet4_baseimage=ghcr.io/geanlabs/gean
ARG devnet4_tag=devnet4

FROM $devnet3_baseimage:$devnet3_tag AS devnet3
FROM $devnet4_baseimage:$devnet4_tag AS devnet4
FROM $devnet3_baseimage:$devnet3_tag

COPY validators.yaml /assets/lean/validators.yaml
COPY --from=devnet3 /usr/local/bin/gean /usr/local/bin/gean-devnet3
COPY --from=devnet4 /usr/local/bin/gean /usr/local/bin/gean-devnet4
ADD gean.sh /gean.sh

RUN chmod +x /gean.sh \
    && (/usr/local/bin/gean-devnet3 --version 2>/dev/null || echo "gean-devnet3") | head -1 > /version-devnet3.txt \
    && (/usr/local/bin/gean-devnet4 --version 2>/dev/null || echo "gean-devnet4") | head -1 > /version-devnet4.txt \
    && printf "gean devnet3: %s\ngean devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/gean.sh"]
