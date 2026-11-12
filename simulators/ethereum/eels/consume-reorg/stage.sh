#!/bin/sh
# Fill the reorg fixtures from a local execution-specs checkout and stage them
# next to Dockerfile.git, for a `--config` entry with `dockerfile: git` and
# `fixtures: /fixtures`. Only needed until a published fixtures release contains
# the `blockchain_test_engine_reorg` format; after that, run the default
# Dockerfile with `--sim.buildarg tag=...` instead.
#
#   stage.sh <execution-specs-checkout> [extra fill args...]
set -eu
SRC=${1:?execution-specs checkout}
shift
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=$(mktemp -d)
(cd "$SRC" && uv run fill tests/reorg --until Amsterdam \
    --output "$OUT/fixtures" --clean -q "$@" >/dev/null)
(cd "$OUT" && COPYFILE_DISABLE=1 tar -czf "$HERE/fixtures.tar.gz" \
    fixtures/.meta fixtures/blockchain_tests_engine_reorg)
rm -rf "$OUT"
ls -la "$HERE/fixtures.tar.gz"
