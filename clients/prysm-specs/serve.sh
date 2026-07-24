#!/bin/bash
set -euo pipefail
cd "${OUT_DIR:-/out}"
exec python3 -m http.server 5151
