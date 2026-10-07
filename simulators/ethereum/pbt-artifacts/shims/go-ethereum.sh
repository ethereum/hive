#!/bin/bash
# go-ethereum. Every verb works on a datadir of its own, so the running node
# is never touched.
set -u
. /hive-bin/pbt-common.sh

GETH=/usr/local/bin/geth
verb="${1:-}"; shift || true

# init_datadir builds a chain from the same genesis the node booted from.
init_datadir() {
    local dir="$1"; shift
    [ -d "$dir" ] || "$GETH" --datadir "$dir" "$@" init /genesis.json >/dev/null 2>&1
}

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    unpack || { echo "cannot unpack the fixtures" >&2; exit 2; }
    init_datadir /pbt/dd-verify || { echo "genesis init failed" >&2; exit 2; }
    [ -f "$FIXTURES/$1" ] && [ -f "$FIXTURES/$2" ] || { echo "fixture file missing" >&2; exit 2; }
    out=$("$GETH" --datadir /pbt/dd-verify bintrie import --verify-only "$FIXTURES/$1" "$FIXTURES/$2" "$3" 2>&1)
    status=$?
    echo "client_exit=$status"
    case $status in
    0) exit 0 ;;
    1) echo "$out" >&2; exit 1 ;;
    *) echo "$out" >&2; exit 2 ;;
    esac
    ;;

convert)
    # The converter reads the plain keys behind the trie paths, so the state
    # is written with the preimage store on. A defect gets a fresh datadir of
    # its own: init_datadir reuses one, and a deleted preimage must not leak
    # into the sound convert.
    shift
    dir=/pbt/dd-convert
    if [ $# -gt 0 ]; then
        dir=/pbt/dd-defect
        rm -rf "$dir"
    fi
    init_datadir "$dir" --cache.preimages || { echo "genesis init failed" >&2; exit 2; }
    while [ $# -gt 0 ]; do
        [ "$1" = drop-preimage ] && [ $# -ge 2 ] || { echo "unknown defect: $*" >&2; exit 2; }
        key=0x7365637572652d6b65792d${2#0x} # "secure-key-" + hash; without 0x, geth reads it as text
        "$GETH" --datadir "$dir" db get "$key" >/dev/null 2>&1 || { echo "no preimage stored under $2" >&2; exit 2; }
        "$GETH" --datadir "$dir" db delete "$key" >/dev/null 2>&1 || { echo "cannot delete $key" >&2; exit 2; }
        shift 2
    done
    out=$("$GETH" --datadir "$dir" bintrie convert --force \
        --snapshot-out /pbt/snapshot.bin --preimages-out /pbt/preimages.bin 2>&1)
    status=$?
    echo "client_exit=$status"
    if [ $status -ne 0 ]; then
        echo "$out" >&2
        # Converter step 2 is the only refusal this verb can be asked for.
        echo "$out" | grep -qE 'missing preimage for (account hash|flat account|storage key|flat storage key)' && exit 1
        exit 2
    fi
    echo "snapshot=$(b64 /pbt/snapshot.bin)"
    echo "preimages=$(b64 /pbt/preimages.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
