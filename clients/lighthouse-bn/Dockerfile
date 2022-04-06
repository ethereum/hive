ARG branch=latest-unstable

FROM sigp/lighthouse:$branch

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]
