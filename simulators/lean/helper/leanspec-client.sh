#!/bin/sh

set -eu

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
case "$DEVNET_LABEL" in
    devnet3)
        LEAN_SPEC_APP_ROOT="${LEAN_SPEC_DEVNET3_ROOT:-/app/devnet3}"
        LEAN_SPEC_KEYS_DIR="${LEAN_SPEC_DEVNET3_KEYS_DIR:-/app/hive/prod_scheme_devnet3}"
        ;;
    devnet4)
        LEAN_SPEC_APP_ROOT="${LEAN_SPEC_DEVNET4_ROOT:-/app/devnet4}"
        LEAN_SPEC_KEYS_DIR="${LEAN_SPEC_DEVNET4_KEYS_DIR:-/app/hive/prod_scheme_devnet4}"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

if [ ! -f "${LEAN_SPEC_KEYS_DIR}/0.json" ] || [ ! -f "${LEAN_SPEC_KEYS_DIR}/1.json" ] || [ ! -f "${LEAN_SPEC_KEYS_DIR}/2.json" ]; then
    echo "Missing LeanSpec helper keys in ${LEAN_SPEC_KEYS_DIR}" >&2
    exit 1
fi

if [ ! -x "${LEAN_SPEC_APP_ROOT}/.venv/bin/python" ]; then
    echo "LeanSpec helper runtime is missing from ${LEAN_SPEC_APP_ROOT}" >&2
    exit 1
fi

if ! "${LEAN_SPEC_APP_ROOT}/.venv/bin/python" -c 'import aiohttp, lean_spec' >/dev/null 2>&1; then
    echo "LeanSpec helper runtime in ${LEAN_SPEC_APP_ROOT} is incomplete" >&2
    exit 1
fi

export VIRTUAL_ENV="${LEAN_SPEC_APP_ROOT}/.venv"
export PATH="${VIRTUAL_ENV}/bin:${PATH}"

cd "${LEAN_SPEC_APP_ROOT}"

echo "Starting lean-spec local helper on ${DEVNET_LABEL} using ${LEAN_SPEC_APP_ROOT}"
exec python /app/hive/lean_spec_client_runner.py
