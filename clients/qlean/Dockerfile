ARG devnet4_baseimage=qdrvm/qlean-mini
ARG devnet4_tag=devnet-4
ARG devnet5_baseimage=qdrvm/qlean-mini
# TODO: this is a placeholder until a devnet5 image is published
ARG devnet5_tag=devnet-4

FROM $devnet5_baseimage:$devnet5_tag AS devnet5
FROM $devnet4_baseimage:$devnet4_tag

RUN cp /opt/qlean/bin/qlean /usr/local/bin/qlean-devnet4
COPY --from=devnet5 /opt/qlean/bin/qlean /usr/local/bin/qlean-devnet5

COPY qlean.sh /qlean.sh
RUN chmod +x /qlean.sh \
    && /usr/local/bin/qlean-devnet4 --version | head -1 > /version-devnet4.txt \
    && /usr/local/bin/qlean-devnet5 --version | head -1 > /version-devnet5.txt \
    && printf "qlean devnet4: %s\nqlean devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/qlean.sh"]
