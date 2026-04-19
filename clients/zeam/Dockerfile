ARG zeam_skip_devnet3=false

FROM denisgolius/zig:0.15.2 AS zig-toolchain

FROM ubuntu:24.04 AS build-base

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

ARG DEBIAN_FRONTEND=noninteractive
ARG RUST_NIGHTLY_TOOLCHAIN=nightly-2026-04-12

RUN rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && apt-get update -o Acquire::Retries=3 -o Acquire::By-Hash=force \
    && apt-get install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    curl \
    git \
    libssl-dev \
    pkg-config \
    python3 \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

COPY --from=zig-toolchain /usr/local/bin/zig /opt/zig

RUN ln -s /opt/zig/zig /usr/local/bin/zig

RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --default-toolchain ${RUST_NIGHTLY_TOOLCHAIN}

ENV PATH="/root/.cargo/bin:${PATH}"

# The devnet3 binary is built from a pinned zeam source revision with the
# `epoll-v2` patch; the full bootstrap (apt + rustup + git clone + zig build)
# adds ~20 minutes to every image build. That cost is only justified when hive
# actually exercises devnet3 -- simulator profiles targeting only devnet4 pay
# it for nothing. The `zeam_skip_devnet3` build arg gates the expensive work:
#
#   - `false` (default): the real source build, unchanged.
#   - `true`: writes a placeholder binary so the downstream COPY still
#     resolves. The stub exits non-zero with a clear message if zeam.sh ever
#     tries to run it at runtime (i.e. a skip-built image is launched with
#     HIVE_LEAN_DEVNET_LABEL=devnet3), so misconfiguration surfaces
#     immediately rather than silently.
#
# The skip is implemented inline within a single `devnet3-builder` stage
# rather than via a stage-selector trick (`FROM stage-${arg} AS ...`) because
# hive's docker client uses the legacy docker build API, which evaluates every
# RUN in every stage regardless of which stage the final image selects. A
# stage split would only work under BuildKit; inline gating works on both.

FROM build-base AS devnet3-builder

ARG zeam_skip_devnet3
ARG zeam_devnet3_revision=1d11cefdcee6b8af81a5ef70c8c7c8dd2f93fb20
ARG zeam_devnet3_patch_revision=epoll-v2

WORKDIR /src

COPY apply-devnet3-patch.py /tmp/apply-devnet3-patch.py

RUN set -eu; \
    if [ "$zeam_skip_devnet3" = "true" ]; then \
        mkdir -p /src/zig-out/bin; \
        printf '#!/bin/sh\necho "zeam-devnet3 binary was not built (image built with zeam_skip_devnet3=true); rebuild without that flag to run devnet3 tests" >&2\nexit 64\n' \
            > /src/zig-out/bin/zeam; \
        chmod +x /src/zig-out/bin/zeam; \
        exit 0; \
    fi; \
    git clone https://github.com/blockblaz/zeam . \
        && git checkout ${zeam_devnet3_revision}; \
    if [ -n "${zeam_devnet3_patch_revision}" ]; then \
        python3 /tmp/apply-devnet3-patch.py; \
    fi; \
    GIT_VERSION=$(git rev-parse --short HEAD); \
    zig build -j1 -Doptimize=ReleaseSafe -Dtarget=x86_64-linux-gnu -Dcpu=baseline -Dgit_version="$GIT_VERSION"

FROM blockblaz/zeam:devnet4 AS devnet4-binary

FROM ubuntu:24.04

COPY --from=devnet3-builder /src/zig-out/bin/zeam /usr/local/bin/zeam-devnet3
COPY --from=devnet4-binary /app/zig-out/bin/zeam /usr/local/bin/zeam-devnet4

ADD zeam.sh /zeam.sh

RUN chmod +x /zeam.sh \
    && (/usr/local/bin/zeam-devnet3 --version || echo "zeam-devnet3") | head -1 > /version-devnet3.txt \
    && (/usr/local/bin/zeam-devnet4 --version || echo "zeam-devnet4") | head -1 > /version-devnet4.txt \
    && printf "zeam devnet3: %s\nzeam devnet4: %s\n" \
        "$(cat /version-devnet3.txt)" \
        "$(cat /version-devnet4.txt)" > /version.txt

EXPOSE 5052 9000/udp 8080

ENTRYPOINT ["/zeam.sh"]
