ARG baseimage=bitminemavan/lantern
ARG devnet4_tag=v0.0.4
ARG devnet5_tag=devnet-5

FROM ${baseimage}:${devnet5_tag} AS devnet5
FROM ${baseimage}:${devnet4_tag}

# The devnet5 binary copied in below is dynamically linked against libcurl,
# libevent and libyaml (lantern's HTTP/storage/YAML refactor, d3751267), but
# this v0.0.4 devnet4 base image predates that refactor and doesn't ship
# them, so the binary aborts at startup with "libcurl.so.4: cannot open
# shared object file". Install the missing runtime libs here.
RUN apt-get update && apt-get install -y --no-install-recommends libcurl4 libevent-2.1-7 libevent-extra-2.1-7 libevent-pthreads-2.1-7 libyaml-0-2 && rm -rf /var/lib/apt/lists/*

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
