#!/usr/bin/env python3
"""Apply the `epoll-v2` patch to the checked-out zeam devnet3 source tree.

Run from the zeam source checkout (cwd = /src in the Dockerfile). This is the
same patch that used to live inline in clients/zeam/Dockerfile; it has been
extracted into a file so the Dockerfile can COPY it and run it conditionally
behind a shell `if`, without fighting the heredoc parser in legacy docker
build.
"""

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
