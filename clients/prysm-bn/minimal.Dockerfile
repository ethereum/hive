ARG baseimage=gcr.io/prysmaticlabs/prysm/beacon-chain
ARG tag=latest

# TODO: either special upstream build, or clone + build minimal version here in dockerfile.
FROM $baseimage:$tag

ADD prysm_bn.sh /prysm_bn.sh
RUN chmod +x /prysm_bn.sh

# TODO: output accurate client version
RUN echo "latest" > /version.txt

ENTRYPOINT ["/prysm_bn.sh"]

