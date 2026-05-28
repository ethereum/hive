ARG baseimage=piertwo/lantern
ARG devnet4_tag=v0.0.4

FROM ${baseimage}:${devnet4_tag}

ADD lantern.sh /lantern.sh

RUN chmod +x /lantern.sh \
    && (/opt/lantern/bin/lantern --version || echo "lantern-devnet4") | head -1 > /version-devnet4.txt \
    && printf "lantern devnet4: %s\n" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/lantern.sh"]
