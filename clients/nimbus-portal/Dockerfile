ARG baseimage=statusim/nimbus-portal-client
ARG tag=amd64-master-latest

FROM $baseimage:$tag

ADD nimbus_portal.sh /nimbus_portal.sh
RUN chmod +x /nimbus_portal.sh

RUN echo "latest" > /version.txt

EXPOSE 8545 9009/udp

ENTRYPOINT ["/nimbus_portal.sh"]
