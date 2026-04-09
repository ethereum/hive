ARG baseimage=piertwo/lantern
ARG tag=v0.0.3

FROM $baseimage:$tag

COPY validators.yaml /assets/lean/validators.yaml
ADD lantern.sh /lantern.sh

RUN chmod +x /lantern.sh \
    && (lantern --version || lantern_cli --version) | head -1 > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/lantern.sh"]