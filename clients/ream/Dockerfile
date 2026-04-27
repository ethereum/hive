ARG devnet3_baseimage=ghcr.io/reamlabs/ream
ARG devnet3_tag=latest-devnet3
ARG devnet4_baseimage=ghcr.io/reamlabs/ream
ARG devnet4_tag=latest-devnet4
ARG devnet4_leanmultisig_rev=703359a8d3bf735c53410280c9efbcaeef02f53e

FROM alpine/git:latest AS devnet4-leanmultisig
ARG devnet4_leanmultisig_rev
RUN git clone https://github.com/leanEthereum/leanMultisig /leanMultisig \
    && cd /leanMultisig \
    && git checkout ${devnet4_leanmultisig_rev}

FROM $devnet4_baseimage:$devnet4_tag AS devnet4
FROM $devnet3_baseimage:$devnet3_tag

COPY validators.yaml /assets/lean/validators.yaml
RUN cp /usr/local/bin/ream /usr/local/bin/ream-devnet3
COPY --from=devnet4 /usr/local/bin/ream /usr/local/bin/ream-devnet4
COPY --from=devnet4-leanmultisig /leanMultisig/crates/rec_aggregation /home/runner/.cargo/git/checkouts/leanmultisig-f4c4eb5eca99429a/703359a/crates/rec_aggregation
ADD ream.sh /ream.sh

RUN chmod +x /ream.sh \
    && /usr/local/bin/ream-devnet3 --version | head -1 > /version-devnet3.txt \
    && /usr/local/bin/ream-devnet4 --version | head -1 > /version-devnet4.txt \
    && printf "ream devnet3: %s\nream devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/ream.sh"]
