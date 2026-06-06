ARG baseimage=piertwo/lantern
ARG devnet4_tag=v0.0.4
ARG devnet5_tag=devnet5

FROM ${baseimage}:${devnet5_tag} AS devnet5
FROM ${baseimage}:${devnet4_tag}

COPY --from=devnet5 /opt/lantern/bin/lantern /usr/local/bin/lantern-devnet5
ADD lantern.sh /lantern.sh

RUN chmod +x /lantern.sh \
    && (/opt/lantern/bin/lantern --version || echo "lantern-devnet4") | head -1 > /version-devnet4.txt \
    && (/usr/local/bin/lantern-devnet5 --version || echo "lantern-devnet5") | head -1 > /version-devnet5.txt \
    && printf "lantern devnet4: %s\nlantern devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/lantern.sh"]
