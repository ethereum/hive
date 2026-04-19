FROM blockblaz/zeam:devnet4 AS devnet4-binary

FROM ubuntu:24.04

COPY --from=devnet4-binary /app/zig-out/bin/zeam /usr/local/bin/zeam

ADD zeam.sh /zeam.sh

RUN chmod +x /zeam.sh \
    && /usr/local/bin/zeam --version | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/zeam.sh"]
