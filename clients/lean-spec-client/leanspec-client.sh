#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"

case "$DEVNET_LABEL" in
    devnet3)
        LEAN_SPEC_APP_ROOT="${LEAN_SPEC_DEVNET3_ROOT:-/app/devnet3}"
        ;;
    devnet4)
        LEAN_SPEC_APP_ROOT="${LEAN_SPEC_DEVNET4_ROOT:-/app/devnet4}"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

export VIRTUAL_ENV="${LEAN_SPEC_APP_ROOT}/.venv"
export PATH="${VIRTUAL_ENV}/bin:${PATH}"

cd "$LEAN_SPEC_APP_ROOT"

attempt=0
while true; do
    attempt=$((attempt + 1))
    echo "Starting lean-spec-client runner attempt ${attempt} on ${DEVNET_LABEL} using ${LEAN_SPEC_APP_ROOT}"

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
