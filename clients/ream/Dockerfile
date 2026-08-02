ARG devnet4_baseimage=ghcr.io/reamlabs/ream
ARG devnet4_tag=latest
ARG devnet5_baseimage=ghcr.io/reamlabs/ream
ARG devnet5_tag=latest-devnet5
ARG runtime_baseimage=ubuntu:24.04

FROM $devnet4_baseimage:$devnet4_tag AS devnet4
FROM $devnet5_baseimage:$devnet5_tag AS devnet5
FROM $runtime_baseimage AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY validators.yaml /assets/lean/validators.yaml
COPY --from=devnet4 /usr/local/bin/ream /usr/local/bin/ream-devnet4
COPY --from=devnet5 /usr/local/bin/ream /usr/local/bin/ream-devnet5
ADD ream.sh /ream.sh

RUN chmod +x /ream.sh \
    && /usr/local/bin/ream-devnet4 --version > /version-devnet4.txt \
    && /usr/local/bin/ream-devnet5 --version > /version-devnet5.txt \
    && printf "ream devnet4: %s\nream devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080
ENTRYPOINT ["/ream.sh"]