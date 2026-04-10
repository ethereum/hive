#!/bin/bash

set -euo pipefail

attempt=0
while true; do
    attempt=$((attempt + 1))
    echo "Starting lean-spec-client runner attempt ${attempt}"

    set +e
    python /app/hive/lean_spec_client_runner.py
    status=$?
    set -e

    if [[ ${status} -eq 0 ]]; then
        exit 0
    fi

    echo "lean-spec-client runner exited with status ${status}; restarting"
    sleep 1
done
