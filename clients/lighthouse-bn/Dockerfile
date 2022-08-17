ARG branch=latest-unstable

FROM sigp/lighthouse:$branch

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

ADD lighthouse_version.sh /lighthouse_version.sh
RUN chmod +x /lighthouse_version.sh
ADD lighthouse_shutdown.sh /hive-bin/shutdown.sh
RUN chmod +x /hive-bin/shutdown.sh

RUN /lighthouse_version.sh > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]
