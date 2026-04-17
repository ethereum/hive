FROM sifrai/lean:devnet-3 AS devnet3
FROM sifrai/lean:devnet-4 AS devnet4
FROM sifrai/lean:devnet-3

COPY --from=devnet3 /usr/local/bin/lean_client /usr/local/bin/grandine-devnet3
COPY --from=devnet4 /usr/local/bin/lean_client /usr/local/bin/grandine-devnet4

ADD grandine.sh /grandine.sh

RUN chmod +x /grandine.sh \
    && /usr/local/bin/grandine-devnet3 --help | head -1 > /version-devnet3.txt \
    && /usr/local/bin/grandine-devnet4 --help | head -1 > /version-devnet4.txt \
    && printf "grandine devnet3: %s\ngrandine devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/grandine.sh"]
