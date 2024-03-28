ARG tag=unstable
ARG baseimage=sifrai/grandine

FROM $baseimage:$tag

ADD grandine.sh /grandine.sh
RUN chmod +x /grandine.sh
RUN grandine --version > /version.txt

ENTRYPOINT ["/grandine.sh"]
