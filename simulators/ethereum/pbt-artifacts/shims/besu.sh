#!/bin/bash
# Besu verifies and converts with `besu storage pbt`, on builds that carry it
# (matkt/besu's glamsterdam-devnet-8-pbt); a stock build answers unsupported.
# Every call gets a fresh datadir, so the running node is never touched.
set -u
. /hive-bin/pbt-common.sh

BESU=/opt/besu/bin/besu
verb="${1:-}"; shift || true

# pbt runs a storage pbt command on a fresh datadir built from the genesis
# besu.sh mapped for the node.
pbt() {
    rm -rf /pbt/besu-dd
    "$BESU" --data-path=/pbt/besu-dd --genesis-file=/genesis.json storage pbt "$@" 2>&1
}

# `storage pbt --help` exits 0 on a stock build too, so look for the command.
supported() {
    "$BESU" storage --help 2>/dev/null | grep -qE '^\s+pbt\b' || { echo "this besu has no storage pbt command" >&2; exit 3; }
}

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    supported
    unpack || { echo "cannot unpack the fixtures" >&2; exit 2; }
    [ -f "$FIXTURES/$1" ] && [ -f "$FIXTURES/$2" ] || { echo "fixture file missing" >&2; exit 2; }
    out=$(pbt verify --snapshot "$FIXTURES/$1" --preimages "$FIXTURES/$2" --anchor "$3")
    status=$?
    echo "client_exit=$status"
    [ $status -eq 0 ] && exit 0
    echo "$out" >&2
    [ $status -eq 1 ] && echo "$out" | grep -q 'EIP-8347 dual-check rejected' && exit 1
    exit 2
    ;;

convert)
    supported
    anchor="$1"; shift
    # Bonsai keeps no preimage store: the preimages come from the genesis alloc.
    [ $# -eq 0 ] || { echo "no preimage store to remove from" >&2; exit 3; }
    rm -rf /pbt/besu-out && mkdir -p /pbt/besu-out
    out=$(pbt convert --preimages-out /pbt/besu-out/preimages.bin --snapshot /pbt/besu-out/snapshot.bin --anchor "$anchor")
    status=$?
    echo "client_exit=$status"
    [ $status -eq 0 ] || { echo "$out" >&2; exit 2; }
    echo "snapshot=$(b64 /pbt/besu-out/snapshot.bin)"
    echo "preimages=$(b64 /pbt/besu-out/preimages.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
