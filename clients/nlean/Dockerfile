ARG devnet4_baseimage=ghcr.io/nleaneth/nlean
ARG devnet4_tag=devnet4
ARG devnet5_baseimage=ghcr.io/nleaneth/nlean
# TODO: this is a placeholder until a devnet5 image is published
ARG devnet5_tag=devnet4

FROM $devnet5_baseimage:$devnet5_tag AS devnet5
FROM $devnet4_baseimage:$devnet4_tag

RUN cp /app/Lean.Client /app/Lean.Client-devnet4
COPY --from=devnet5 /app/Lean.Client /app/Lean.Client-devnet5
ADD nlean.sh /nlean.sh

RUN chmod +x /nlean.sh \
    && /app/Lean.Client-devnet4 --version | head -1 > /version-devnet4.txt \
    && /app/Lean.Client-devnet5 --version | head -1 > /version-devnet5.txt \
    && printf "nlean devnet4: %s\nnlean devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/nlean.sh"]
