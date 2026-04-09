ARG baseimage=qdrvm/qlean-mini
ARG tag=devnet-3

FROM $baseimage:$tag

COPY validators.yaml /assets/lean/validators.yaml
ADD qlean.sh /qlean.sh

RUN chmod +x /qlean.sh \
    && qlean --version | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/qlean.sh"]
