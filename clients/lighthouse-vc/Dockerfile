ARG branch=latest-unstable

FROM sigp/lighthouse:$branch

ADD lighthouse_vc.sh /lighthouse_vc.sh
RUN chmod +x /lighthouse_vc.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/lighthouse_vc.sh"]
