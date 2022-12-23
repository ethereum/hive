ARG branch=HEAD-29dfca
FROM gcr.io/prysmaticlabs/prysm/validator:$branch as upstream

FROM debian:buster-slim
COPY --from=upstream /app/cmd/validator/validator validator

ADD prysm_vc.sh /prysm_vc.sh
RUN chmod +x /prysm_vc.sh

RUN /validator --version > /version.txt

ENTRYPOINT ["/prysm_vc.sh"]
