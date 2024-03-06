ARG tag=unstable
ARG baseimage=ethpandaops/nimbus-validator-client

FROM $baseimage:$tag AS source

FROM debian:bookworm-slim AS deploy

RUN apt-get update \
 && apt-get install -y librocksdb-dev bash curl jq\
 && apt-get clean \
 && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

COPY --from=source /home/user/nimbus-eth2/build/nimbus_validator_client /usr/bin/nimbus_validator_client

# Add the startup script.
ADD nimbus_vc.sh /nimbus_vc.sh
RUN chmod +x /nimbus_vc.sh

ADD nimbus_version.sh /nimbus_version.sh
RUN chmod +x /nimbus_version.sh

RUN /nimbus_version.sh > /version.txt

ENTRYPOINT ["/nimbus_vc.sh"]
