ARG baseimage=piertwo/lantern
ARG devnet3_tag=v0.0.3
ARG devnet4_tag=v0.0.4

FROM ${baseimage}:${devnet3_tag} AS devnet3-binary
FROM ${baseimage}:${devnet4_tag} AS devnet4-binary
FROM ${baseimage}:${devnet4_tag}

COPY --from=devnet3-binary /opt/lantern /opt/lantern-devnet3

ADD lantern.sh /lantern.sh

RUN chmod +x /lantern.sh \
    && (LD_LIBRARY_PATH=/opt/lantern-devnet3/lib /opt/lantern-devnet3/bin/lantern --version || echo "lantern-devnet3") | head -1 > /version-devnet3.txt \
    && (/opt/lantern/bin/lantern --version || echo "lantern-devnet4") | head -1 > /version-devnet4.txt \
    && printf "lantern devnet3: %s\nlantern devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/lantern.sh"]
