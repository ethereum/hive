ARG baseimage=blockblaz/zeam
ARG tag=devnet3

FROM ubuntu:24.04

COPY --from=blockblaz/zeam:devnet3 /app/zig-out/bin/zeam /usr/local/bin/zeam

COPY validators.yaml /assets/lean/validators.yaml
ADD zeam.sh /zeam.sh

RUN chmod +x /zeam.sh \
    && (zeam --version || echo "zeam-devnet3") | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/zeam.sh"]
