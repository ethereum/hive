ARG devnet3_baseimage=qdrvm/qlean-mini
ARG devnet3_tag=devnet-3
ARG devnet4_baseimage=qdrvm/qlean-mini
ARG devnet4_tag=devnet-4

FROM $devnet4_baseimage:$devnet4_tag AS devnet4

FROM $devnet3_baseimage:$devnet3_tag

RUN mkdir -p /opt/qlean-d3/bin && \
    cp -a /opt/qlean/. /opt/qlean-d3/ || true && \
    (cp /usr/local/bin/qlean /opt/qlean-d3/bin/qlean-devnet3 || \
     cp $(find /opt/qlean-d3 -name qlean -type f -executable | head -n 1) /opt/qlean-d3/bin/qlean-devnet3)

RUN mkdir -p /opt/qlean-d4/bin
COPY --from=devnet4 /opt/qlean /opt/qlean-d4
RUN (cp $(find /opt/qlean-d4 -name qlean -type f -executable | head -n 1) /opt/qlean-d4/bin/qlean-devnet4)

COPY qlean.sh /qlean.sh
RUN chmod +x /qlean.sh \
    && qlean --version | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/qlean.sh"]
