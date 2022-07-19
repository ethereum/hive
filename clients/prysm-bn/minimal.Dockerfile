ARG branch=latest

# TODO: either special upstream build, or clone + build minimal version here in dockerfile.
FROM gcr.io/prysmaticlabs/prysm/beacon-chain:$branch

ADD prysm_bn.sh /prysm_bn.sh
RUN chmod +x /prysm_bn.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/prysm_bn.sh"]

