#!/bin/bash

set -euo pipefail

python /app/hive/prepare_leanspec_assets.py
exec python /app/hive/lean_spec_client_runner.py
