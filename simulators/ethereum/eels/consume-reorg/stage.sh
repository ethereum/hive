#!/bin/sh
# Fill the reorg fixtures from a local execution-specs checkout and stage them
# next to this Dockerfile, for use with `--sim.buildarg fixtures=/fixtures`.
# Only needed until a published fixtures release contains the
# `blockchain_test_engine_reorg` format; after that, pass a release or URL to
# `--sim.buildarg fixtures=...` instead.
#
#   stage.sh <execution-specs-checkout> [extra fill args...]
set -eu
SRC=${1:?execution-specs checkout}
shift
HERE=$(cd "$(dirname "$0")" && pwd)
OUT=$(mktemp -d)
(cd "$SRC" && uv run fill tests/reorg --from Shanghai --until Amsterdam \
    --output "$OUT/fixtures" --clean -q "$@" >/dev/null)
(cd "$OUT" && COPYFILE_DISABLE=1 tar -czf "$HERE/fixtures.tar.gz" \
    fixtures/.meta fixtures/blockchain_tests_engine_reorg)
rm -rf "$OUT"
ls -la "$HERE/fixtures.tar.gz"
