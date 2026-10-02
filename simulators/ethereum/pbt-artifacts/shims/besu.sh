#!/bin/bash
# Besu: genesis-root via RPC; verify via `storage pbt verify`; convert via
# `storage pbt convert` (preimages from genesis alloc + snapshot from anchor state).
set -u
. /hive-bin/pbt-common.sh

BESU=/opt/besu/bin/besu
verb="${1:-}"; shift || true

# Fresh datadir so offline verbs do not fight the running node's RocksDB lock.
# Uses the same mapped /genesis.json the hive node booted from.
init_datadir() {
    local dir="$1"
    rm -rf "$dir"
    mkdir -p "$dir"
}

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    unpack || { echo "cannot unpack the fixtures" >&2; exit 2; }
    [ $# -ge 3 ] || { echo "verify needs <snapshot> <preimages> <anchor>" >&2; exit 2; }
    snapshot="$FIXTURES/$1"
    preimages="$FIXTURES/$2"
    anchor="$3"
    init_datadir /pbt/dd-verify || { echo "cannot create verify datadir" >&2; exit 2; }
    out=$("$BESU" \
        --data-path=/pbt/dd-verify \
        --genesis-file=/genesis.json \
        --data-storage-format=BONSAI \
        --network-id=1337 \
        --logging=WARN \
        storage pbt verify \
        --snapshot="$snapshot" \
        --preimages="$preimages" \
        --anchor="$anchor" 2>&1)
    status=$?
    echo "client_exit=$status"
    if [ $status -eq 0 ]; then
        exit 0
    fi
    echo "$out" >&2
    # Picocli maps ExecutionException to exit 1; treat reject: and that exit as reject.
    if [ $status -eq 1 ] || echo "$out" | grep -qiE '^reject:|invalid preimage'; then
        exit 1
    fi
    exit 2
    ;;

convert)
    # Hive: convert <anchor> [drop-preimage <0xhash>]...
    # Besu derives preimages from genesis alloc (no keccak preimage store to mutate),
    # so drop-preimage defects are unsupported.
    [ $# -ge 1 ] || { echo "convert needs <anchor>" >&2; exit 2; }
    anchor="$1"
    shift
    if [ $# -gt 0 ]; then
        echo "besu has no mutable preimage store for drop-preimage defects" >&2
        exit 3
    fi
    init_datadir /pbt/dd-convert || { echo "cannot create convert datadir" >&2; exit 2; }
    mkdir -p /pbt/out
    out=$("$BESU" \
        --data-path=/pbt/dd-convert \
        --genesis-file=/genesis.json \
        --data-storage-format=BONSAI \
        --network-id=1337 \
        --logging=WARN \
        storage pbt convert \
        --snapshot=/pbt/out/snapshot.bin \
        --preimages-out=/pbt/out/preimages.bin \
        --anchor="$anchor" 2>&1)
    status=$?
    echo "client_exit=$status"
    if [ $status -ne 0 ]; then
        echo "$out" >&2
        if [ $status -eq 1 ] || echo "$out" | grep -qiE 'rejected|reject:'; then
            exit 1
        fi
        exit 2
    fi
    echo "snapshot=$(b64 /pbt/out/snapshot.bin)"
    echo "preimages=$(b64 /pbt/out/preimages.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
