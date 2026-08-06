ARG devnet5_baseimage=ghcr.io/reamlabs/ream
ARG devnet5_tag=latest
ARG runtime_baseimage=ubuntu:24.04

FROM $devnet5_baseimage:$devnet5_tag AS devnet5
FROM $runtime_baseimage AS runtime

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY validators.yaml /assets/lean/validators.yaml
COPY --from=devnet5 /usr/local/bin/ream /usr/local/bin/ream-devnet5
ADD ream.sh /ream.sh

RUN chmod +x /ream.sh \
    && /usr/local/bin/ream-devnet5 --version > /version-devnet5.txt \
    && printf "ream devnet5: %s\n" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080
ENTRYPOINT ["/ream.sh"]
