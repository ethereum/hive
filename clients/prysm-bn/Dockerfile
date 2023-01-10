ARG branch=capella-6b2721
FROM gcr.io/prysmaticlabs/prysm/beacon-chain:$branch as upstream

FROM debian:buster-slim
COPY --from=upstream /app/cmd/beacon-chain/beacon-chain beacon-chain

ADD prysm_bn.sh /prysm_bn.sh
RUN chmod +x /prysm_bn.sh

RUN /beacon-chain --version > /version.txt

ENTRYPOINT ["/prysm_bn.sh"]
