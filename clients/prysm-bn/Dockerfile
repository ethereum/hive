ARG branch=latest-debug
FROM gcr.io/prysmaticlabs/prysm/beacon-chain:$branch as upstream

FROM debian:buster-slim
COPY --from=upstream /app/cmd/beacon-chain/beacon-chain beacon-chain

ADD prysm_bn.sh /prysm_bn.sh
RUN chmod +x /prysm_bn.sh
ADD prysm_shutdown.sh /hive-bin/shutdown.sh
RUN chmod +x /hive-bin/shutdown.sh

RUN apt-get update -y && apt-get install -y procps

RUN /beacon-chain --version > /version.txt

ENTRYPOINT ["/prysm_bn.sh"]
