ARG branch=latest

FROM sigp/lighthouse:$branch

ADD make_config.sh /make_config.sh
RUN chmod +x /make_config.sh

ADD lighthouse_bn.sh /lighthouse_vc.sh
RUN chmod +x /lighthouse_vc.sh

# TODO: output client version

ENTRYPOINT ["/lighthouse_vc.sh"]
