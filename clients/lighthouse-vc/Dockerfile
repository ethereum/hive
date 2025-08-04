ARG tag=latest-unstable
ARG baseimage=sigp/lighthouse

FROM $baseimage:$tag

ADD lighthouse_vc.sh /lighthouse_vc.sh
RUN chmod +x /lighthouse_vc.sh

ADD lighthouse_version.sh /lighthouse_version.sh
RUN chmod +x /lighthouse_version.sh

RUN /lighthouse_version.sh > /version.txt

ENTRYPOINT ["/lighthouse_vc.sh"]
