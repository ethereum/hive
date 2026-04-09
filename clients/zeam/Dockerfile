ARG baseimage=blockblaz/zeam
ARG tag=devnet3

FROM alpine:latest AS builder
RUN echo "zeam-devnet3" > /version.txt

RUN mkdir -p /export/assets/lean
COPY validators.yaml /export/assets/lean/validators.yaml
COPY zeam.sh /export/zeam.sh
RUN chmod +x /export/zeam.sh

FROM ${baseimage}:${tag}

COPY --from=builder /version.txt /version.txt
COPY --from=builder /export/ /

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/zeam.sh"]
