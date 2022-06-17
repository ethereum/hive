ARG branch=develop

FROM consensys/teku:$branch

USER root

ADD teku_bn.sh /opt/teku/bin/teku_bn.sh
RUN chmod +x /opt/teku/bin//teku_bn.sh

RUN ./bin/teku --version > /version.txt > /version.txt

ENTRYPOINT ["/opt/teku/bin/teku_bn.sh"]
