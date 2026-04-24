ARG devnet3_baseimage=sifrai/lean
ARG devnet3_tag=devnet-3
ARG devnet4_baseimage=sifrai/lean
ARG devnet4_tag=devnet-4

FROM $devnet3_baseimage:$devnet3_tag AS devnet3
FROM $devnet4_baseimage:$devnet4_tag AS devnet4

FROM $devnet3_baseimage:$devnet3_tag

COPY validators.yaml /assets/lean/validators.yaml
COPY --from=devnet3 /usr/local/bin/lean_client /usr/local/bin/lean_client-devnet3
COPY --from=devnet4 /usr/local/bin/lean_client /usr/local/bin/lean_client-devnet4
ADD grandine_lean.sh /grandine_lean.sh

RUN chmod +x /grandine_lean.sh \
    && /usr/local/bin/lean_client-devnet3 --version | head -1 > /version-devnet3.txt \
    && /usr/local/bin/lean_client-devnet4 --version | head -1 > /version-devnet4.txt \
    && printf "grandine_lean devnet3: %s\ngrandine_lean devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/grandine_lean.sh"]
