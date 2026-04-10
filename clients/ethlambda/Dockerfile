ARG baseimage=ghcr.io/lambdaclass/ethlambda
ARG tag=devnet3

FROM $baseimage:$tag

COPY validators.yaml /assets/lean/validators.yaml
ADD ethlambda.sh /ethlambda.sh

RUN chmod +x /ethlambda.sh \
    && ethlambda --version | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/ethlambda.sh"]
