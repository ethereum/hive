#!/bin/bash
# Erigon reads the node's own datadir read-only: verify-pbt checks a pair
# against block 0's header, export-pbt writes both artifacts.
set -u
. /hive-bin/pbt-common.sh

ERIGON=/usr/local/bin/erigon
verb="${1:-}"; shift || true

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    unpack || { echo "cannot unpack the fixtures" >&2; exit 2; }
    [ -f "$FIXTURES/$1" ] && [ -f "$FIXTURES/$2" ] || { echo "fixture file missing" >&2; exit 2; }
    rm -rf /pbt/verify-tmp
    out=$("$ERIGON" --datadir /erigon-hive-datadir snapshots verify-pbt --tmpdir /pbt/verify-tmp \
        --snapshot "$FIXTURES/$1" --preimages "$FIXTURES/$2" --block "$3" 2>&1)
    status=$?
    echo "client_exit=$status"
    [ $status -eq 0 ] && exit 0
    echo "$out" >&2
    [ $status -eq 1 ] && echo "$out" | grep -q 'verify-pbt: artifact rejected' && exit 1
    exit 2
    ;;

convert)
    if [ $# -gt 1 ]; then
        echo "plain-key state: no preimage store to remove from" >&2
        exit 3
    fi
    # From the node's own datadir: `erigon init` leaves neither the aggregator
    # salt nor a commitment-state record, only a started node does. The export
    # takes its own read-only transaction.
    rm -rf /pbt/out && mkdir -p /pbt/out
    out=$("$ERIGON" --datadir /erigon-hive-datadir snapshots export-pbt --out /pbt/out 2>&1)
    status=$?
    echo "client_exit=$status"
    if [ $status -ne 0 ] || [ ! -f /pbt/out/pbt-snapshot.bin ] || [ ! -f /pbt/out/framed.bin ]; then
        echo "$out" >&2
        exit 2
    fi
    echo "snapshot=$(b64 /pbt/out/pbt-snapshot.bin)"
    echo "preimages=$(b64 /pbt/out/framed.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
