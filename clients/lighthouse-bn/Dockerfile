ARG tag=latest-unstable
ARG baseimage=sigp/lighthouse

FROM $baseimage:$tag

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

ADD lighthouse_version.sh /lighthouse_version.sh
RUN chmod +x /lighthouse_version.sh

RUN apt-get update -y && apt-get install -y xxd

RUN /lighthouse_version.sh > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]
