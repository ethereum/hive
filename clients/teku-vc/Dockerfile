ARG branch=develop

FROM consensys/teku:$branch

USER root

ADD teku_vc.sh /opt/teku/bin/teku_vc.sh
RUN chmod +x /opt/teku/bin/teku_vc.sh

RUN ./bin/teku --version > /version.txt > /version.txt

ENTRYPOINT ["/opt/teku/bin/teku_vc.sh"]
