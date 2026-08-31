#!/bin/bash
# Serve /out (cl-meta.json, junit/, status, *.log) over HTTP so the cl
# simulator can fetch results. Replaces the entrypoint process; the
# container stays alive until hive stops it.
set -euo pipefail

cd "${OUT_DIR:-/out}"
exec python3 -m http.server 5151
