ARG devnet4_baseimage=ghcr.io/nleaneth/nlean
ARG devnet4_tag=devnet4

FROM $devnet4_baseimage:$devnet4_tag

ADD nlean.sh /nlean.sh

RUN chmod +x /nlean.sh \
    && /app/Lean.Client --version | head -1 > /version-devnet4.txt \
    && printf "nlean devnet4: %s\n" "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/nlean.sh"]
