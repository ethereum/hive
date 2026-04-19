# Global build arg: select which devnet3 builder stage to pull from. Declared
# before the first FROM so it is visible to the `FROM devnet3-builder-${...}`
# selector near the bottom of the file. See the block comment above that
# selector for why the split exists.
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
# `epoll-v2` patch; the full toolchain bootstrap (apt + rustup + git clone +
# zig build) adds ~20 minutes to every image build. That cost is only
# justified when hive actually exercises devnet3 -- simulator profiles
# targeting only devnet4 pay it for nothing. Split the builder into two
# stages and let the caller pick at build time via `zeam_skip_devnet3`:
#
#   - devnet3-builder-false (default): the real source build, unchanged.
#   - devnet3-builder-true: a cheap stub that writes a placeholder binary
#     so the final-stage COPY still resolves. The stub exits non-zero with
#     a clear message if zeam.sh ever tries to run it at runtime (i.e. a
#     skip-built image is launched with HIVE_LEAN_DEVNET_LABEL=devnet3),
#     so misconfiguration surfaces immediately rather than silently.
#
# The final `FROM devnet3-builder-${zeam_skip_devnet3} AS devnet3-builder`
# indirection keeps the downstream COPY untouched regardless of the choice.

FROM build-base AS devnet3-builder-false

ARG zeam_devnet3_revision=1d11cefdcee6b8af81a5ef70c8c7c8dd2f93fb20
ARG zeam_devnet3_patch_revision=epoll-v2

WORKDIR /src

RUN git clone https://github.com/blockblaz/zeam . \
    && git checkout ${zeam_devnet3_revision}

RUN test -n "${zeam_devnet3_patch_revision}" && python3 - <<'PY'
import re
from pathlib import Path

runtime_xev_files = [
    Path("/src/pkgs/cli/src/main.zig"),
    Path("/src/pkgs/cli/src/node.zig"),
    Path("/src/pkgs/network/src/interface.zig"),
    Path("/src/pkgs/network/src/mock.zig"),
    Path("/src/pkgs/network/src/ethlibp2p.zig"),
    Path("/src/pkgs/node/src/clock.zig"),
    Path("/src/pkgs/node/src/node.zig"),
    Path("/src/pkgs/node/src/testing.zig"),
    Path("/src/pkgs/node/src/utils.zig"),
]

for path in runtime_xev_files:
    text = path.read_text()
    text = text.replace('const xev = @import("xev");', 'const xev = @import("xev").Epoll;')
    path.write_text(text)

utils_path = Path("/src/pkgs/node/src/utils.zig")
utils = utils_path.read_text()
new_detect = """/// Detect the best available I/O backend at runtime.
/// Devnet3 is pinned to epoll so startup works under Docker's default seccomp profile.
pub fn detectBackend() !void {
    if (@hasDecl(xev, "prefer")) {
        if (xev.prefer(.epoll)) {
            return;
        }
    }
    if (@hasDecl(xev, "detect")) {
        try xev.detect();
    }
}

"""
if "pub fn detectBackend() !void {" in utils:
    start = utils.index("pub fn detectBackend() !void {")
    end = utils.index("pub const EventLoop = struct {")
    utils = utils[:start] + new_detect + utils[end:]
else:
    anchor = 'const types = @import("@zeam/types");\n\n'
    if anchor not in utils:
        raise SystemExit("utils anchor not found")
    utils = utils.replace(anchor, anchor + new_detect, 1)
utils_path.write_text(utils)

lib_path = Path("/src/pkgs/node/src/lib.zig")
lib = lib_path.read_text()
export_line = "pub const detectBackend = utils.detectBackend;\n"
if export_line not in lib:
    lib = lib.replace(
        'pub const utils = @import("./utils.zig");\n',
        'pub const utils = @import("./utils.zig");\n' + export_line,
        1,
    )
lib_path.write_text(lib)

main_path = Path("/src/pkgs/cli/src/main.zig")
main = main_path.read_text()
detect_block = """    node_lib.detectBackend() catch |err| {
        ErrorHandler.logErrorWithOperation(err, "detect I/O backend");
        return err;
    };

"""
if 'node_lib.detectBackend() catch |err|' not in main:
    marker = "    switch (opts.args.__commands__) {\n"
    if marker not in main:
        raise SystemExit("main switch marker not found")
    main = main.replace(marker, detect_block + marker, 1)
main_path.write_text(main)

build_zig_path = Path("/src/build.zig")
build_zig = build_zig_path.read_text()
build_zig, replaced = re.subn(
    r"(?m)^\s*try build_zkvm_targets\(b, &cli_exe\.step, target, build_options_module, use_poseidon\);\n",
    "    // Hive only needs the zeam CLI binary; skip zkVM artifact builds here.\n",
    build_zig,
    count=1,
)
if replaced != 1:
    raise SystemExit("build_zkvm_targets call not found")
build_zig_path.write_text(build_zig)
PY

RUN GIT_VERSION=$(git rev-parse --short HEAD) \
    && zig build -j1 -Doptimize=ReleaseSafe -Dtarget=x86_64-linux-gnu -Dcpu=baseline -Dgit_version="$GIT_VERSION"

FROM ubuntu:24.04 AS devnet3-builder-true
RUN mkdir -p /src/zig-out/bin \
    && printf '#!/bin/sh\necho "zeam-devnet3 binary was not built (image built with zeam_skip_devnet3=true); rebuild without that flag to run devnet3 tests" >&2\nexit 64\n' \
        > /src/zig-out/bin/zeam \
    && chmod +x /src/zig-out/bin/zeam

FROM devnet3-builder-${zeam_skip_devnet3} AS devnet3-builder

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
