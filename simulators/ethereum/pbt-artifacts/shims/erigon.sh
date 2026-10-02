#!/bin/bash
# Erigon produces the preimage file and consumes neither artifact. Its
# snapshot-side conversion rewrites its own commitment domain in place, which
# is not the EIP's artifact, and it has no importer.
set -u
. /hive-bin/pbt-common.sh

ERIGON=/usr/local/bin/erigon
verb="${1:-}"; shift || true

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    echo "erigon has no importer for an externally produced snapshot or preimage file" >&2
    exit 3
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
    out=$("$ERIGON" --datadir /erigon-hive-datadir snapshots export-preimages --out /pbt/out 2>&1)
    status=$?
    echo "client_exit=$status"
    if [ $status -ne 0 ] || [ ! -f /pbt/out/framed.bin ]; then
        echo "$out" >&2
        exit 2
    fi
    echo "preimages=$(b64 /pbt/out/framed.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
