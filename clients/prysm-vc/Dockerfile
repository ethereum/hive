ARG branch=latest-alpine
FROM gcr.io/prysmaticlabs/prysm/validator:$branch

ADD prysm_vc.sh /prysm_vc.sh
RUN chmod +x /prysm_vc.sh

RUN /app/cmd/validator/validator --version > /version.txt

ENTRYPOINT ["/prysm_vc.sh"]
