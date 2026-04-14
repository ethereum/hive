FROM ghcr.io/lambdaclass/ethlambda:devnet3 AS devnet3
FROM ghcr.io/lambdaclass/ethlambda:devnet4 AS devnet4
FROM ghcr.io/lambdaclass/ethlambda:devnet3

COPY --from=devnet3 /usr/local/bin/ethlambda /usr/local/bin/ethlambda-devnet3
COPY --from=devnet4 /usr/local/bin/ethlambda /usr/local/bin/ethlambda-devnet4
ADD ethlambda.sh /ethlambda.sh

RUN chmod +x /ethlambda.sh \
    && /usr/local/bin/ethlambda-devnet3 --version | head -1 > /version-devnet3.txt \
    && /usr/local/bin/ethlambda-devnet4 --version | head -1 > /version-devnet4.txt \
    && printf "ethlambda devnet3: %s\nethlambda devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/ethlambda.sh"]
