ARG devnet4_baseimage=sifrai/lean
ARG devnet4_tag=devnet-4
ARG devnet5_baseimage=sifrai/lean
# TODO: this is a placeholder until a devnet5 image is published
ARG devnet5_tag=devnet-4

FROM $devnet5_baseimage:$devnet5_tag AS devnet5
FROM $devnet4_baseimage:$devnet4_tag

COPY validators.yaml /assets/lean/validators.yaml
RUN cp /usr/local/bin/lean_client /usr/local/bin/lean_client-devnet4
COPY --from=devnet5 /usr/local/bin/lean_client /usr/local/bin/lean_client-devnet5
ADD grandine_lean.sh /grandine_lean.sh

RUN chmod +x /grandine_lean.sh \
    && /usr/local/bin/lean_client-devnet4 --version | head -1 > /version-devnet4.txt \
    && /usr/local/bin/lean_client-devnet5 --version | head -1 > /version-devnet5.txt \
    && printf "grandine_lean devnet4: %s\ngrandine_lean devnet5: %s\n" \
        "$(cat /version-devnet4.txt)" \
        "$(cat /version-devnet5.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/grandine_lean.sh"]
