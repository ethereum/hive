ARG branch=latest-alpine
FROM gcr.io/prysmaticlabs/prysm/beacon-chain:$branch

ADD prysm_bn.sh /prysm_bn.sh
RUN chmod +x /prysm_bn.sh

RUN /app/cmd/beacon-chain/beacon-chain --version > /version.txt

ENTRYPOINT ["/prysm_bn.sh"]
