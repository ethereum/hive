FROM ghcr.io/lambdaclass/ethlambda:devnet4 AS devnet4
FROM ghcr.io/lambdaclass/ethlambda:devnet5 AS devnet5
FROM ubuntu:24.04

COPY --from=devnet4 /usr/local/bin/ethlambda /usr/local/bin/ethlambda-devnet4
COPY --from=devnet5 /usr/local/bin/ethlambda /usr/local/bin/ethlambda-devnet5
ADD ethlambda.sh /ethlambda.sh

RUN chmod +x /ethlambda.sh \
    && /usr/local/bin/ethlambda-devnet4 --version | head -1 > /version-devnet4.txt \
    && /usr/local/bin/ethlambda-devnet5 --version | head -1 > /version-devnet5.txt \
    && printf "ethlambda devnet4: %s\nethlambda devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/ethlambda.sh"]
