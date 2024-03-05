ARG tag=unstable
ARG baseimage=ethpandaops/nimbus-eth2

FROM $baseimage:$tag AS source

FROM debian:bookworm-slim AS deploy

RUN apt-get update \
 && apt-get install -y librocksdb-dev bash curl jq\
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=source /home/user/nimbus-eth2/build/nimbus_beacon_node /usr/bin/nimbus_beacon_node

RUN usr/bin/nimbus_beacon_node --version > /version.txt

ADD nimbus_bn.sh /nimbus_bn.sh
RUN chmod +x /nimbus_bn.sh
ADD nimbus_version.sh /nimbus_version.sh
RUN chmod +x /nimbus_version.sh

RUN /nimbus_version.sh > /version.txt

EXPOSE 9000 9000/udp 4000 4000/udp

ENTRYPOINT ["/nimbus_bn.sh"]
