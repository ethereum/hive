ARG baseimage=ghcr.io/leanethereum/leanspec-node
ARG tag=latest

FROM $baseimage:$tag

USER root

COPY prepare_leanspec_assets.py /app/hive/prepare_leanspec_assets.py
COPY leanspec-client.sh /app/hive/leanspec-client.sh

RUN chown -R leanspec:leanspec /app/hive \
    && chmod +x /app/hive/leanspec-client.sh \
    && printf "lean-spec-client\n" > /version.txt

USER leanspec

ENV LEAN_ENV=prod

RUN uv run python -m consensus_testing.keys --download --scheme prod

EXPOSE 5052 9001/udp

ENTRYPOINT ["/app/hive/leanspec-client.sh"]
