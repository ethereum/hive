ARG devnet4_baseimage=qdrvm/qlean-mini
ARG devnet4_tag=devnet-4

FROM $devnet4_baseimage:$devnet4_tag

RUN cp /opt/qlean/bin/qlean /usr/local/bin/qlean-devnet4

COPY qlean.sh /qlean.sh
RUN chmod +x /qlean.sh \
    && /usr/local/bin/qlean-devnet4 --version | head -1 > /version-devnet4.txt \
    && printf "qlean devnet4: %s\n" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/qlean.sh"]
