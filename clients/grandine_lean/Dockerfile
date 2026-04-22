ARG devnet3_baseimage=sifrai/lean
ARG devnet3_tag=devnet-3
ARG devnet3_repo=https://github.com/grandinetech/lean
ARG devnet3_branch=devnet-3
ARG devnet3_rev=8f5369e8934c3a98a68e1de53c38265ac91d3c94
ARG devnet3_rust_toolchain=1.94.0
ARG devnet4_repo=https://github.com/grandinetech/lean
ARG devnet4_branch=devnet-4
ARG devnet4_rev=44ed9f5105ee220d097278fa766a8b9422d389da
ARG devnet4_rust_toolchain=1.94.0

FROM rust:1.94-bookworm AS devnet3-builder

ARG devnet3_repo
ARG devnet3_branch
ARG devnet3_rev
ARG devnet3_rust_toolchain

COPY patches/devnet3-hive-compat.patch /tmp/devnet3-hive-compat.patch

WORKDIR /build

RUN git clone --depth 1 --branch ${devnet3_branch} ${devnet3_repo} source \
    && cd source \
    && git checkout ${devnet3_rev} \
    && git apply /tmp/devnet3-hive-compat.patch

WORKDIR /build/source/lean_client

RUN cargo +${devnet3_rust_toolchain} build --locked --release -p lean_client

FROM rust:1.94-bookworm AS devnet4-builder

ARG devnet4_repo
ARG devnet4_branch
ARG devnet4_rev
ARG devnet4_rust_toolchain

COPY patches/devnet4-hive-compat.patch /tmp/devnet4-hive-compat.patch

WORKDIR /build

RUN git clone --depth 1 --branch ${devnet4_branch} ${devnet4_repo} source \
    && cd source \
    && git checkout ${devnet4_rev} \
    && git apply /tmp/devnet4-hive-compat.patch

WORKDIR /build/source/lean_client

RUN cargo +${devnet4_rust_toolchain} build --locked --release -p lean_client

FROM $devnet3_baseimage:$devnet3_tag

COPY validators.yaml /assets/lean/validators.yaml
COPY --from=devnet3-builder /build/source/lean_client/target/release/lean_client /usr/local/bin/lean_client-devnet3
COPY --from=devnet4-builder /build/source/lean_client/target/release/lean_client /usr/local/bin/lean_client-devnet4
ADD grandine_lean.sh /grandine_lean.sh

RUN chmod +x /grandine_lean.sh \
    && /usr/local/bin/lean_client-devnet3 --version | head -1 > /version-devnet3.txt \
    && /usr/local/bin/lean_client-devnet4 --version | head -1 > /version-devnet4.txt \
    && printf "grandine_lean devnet3: %s\ngrandine_lean devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/grandine_lean.sh"]
