ARG tag=develop
ARG baseimage=consensys/teku

FROM $baseimage:$tag

USER root

ADD teku_bn.sh /opt/teku/bin/teku_bn.sh
RUN chmod +x /opt/teku/bin//teku_bn.sh

RUN ./bin/teku --version > /version.txt > /version.txt

ADD trusted_setup.txt /trusted_setup.txt

ENTRYPOINT ["/opt/teku/bin/teku_bn.sh"]
