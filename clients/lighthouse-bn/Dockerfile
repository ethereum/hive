ARG branch=latest

FROM sigp/lighthouse:$branch

ADD make_config.sh /make_config.sh
RUN chmod +x /make_config.sh

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

ADD enr.sh /hive-bin/enr.sh
RUN chmod +x /hive-bin/enr.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]

