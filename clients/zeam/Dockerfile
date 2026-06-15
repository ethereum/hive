FROM blockblaz/zeam:devnet4 AS devnet4-binary
FROM blockblaz/zeam:devnet5 AS devnet5-binary

FROM ubuntu:24.04

COPY --from=devnet4-binary /app/zig-out/bin/zeam /usr/local/bin/zeam-devnet4
COPY --from=devnet5-binary /app/zig-out/bin/zeam /usr/local/bin/zeam-devnet5

ADD zeam.sh /zeam.sh

RUN chmod +x /zeam.sh \
    && /usr/local/bin/zeam-devnet4 --version | head -1 > /version-devnet4.txt \
    && /usr/local/bin/zeam-devnet5 --version | head -1 > /version-devnet5.txt \
    && printf "zeam devnet4: %s\nzeam devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/zeam.sh"]
