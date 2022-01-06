ARG branch=latest

# TODO: either special upstream build, or clone + build minimal version here in dockerfile.
FROM sigp/lighthouse_minimal:$branch

ADD lighthouse_bn.sh /lighthouse_bn.sh
RUN chmod +x /lighthouse_bn.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/lighthouse_bn.sh"]

