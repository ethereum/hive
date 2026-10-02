#!/bin/bash
# Nethermind consumes the artifacts during node startup, so verify boots a
# throwaway node against them and reads the outcome off its log; convert
# boots another that exports both and exits. Written against
# NethermindEth/nethermind's update-pbt-snapshot-format (clients.yaml pins
# it); see README.md for the chainspec it synthesizes.
set -u
. /hive-bin/pbt-common.sh

NETHERMIND=/nethermind/nethermind
verb="${1:-}"; shift || true

# All three after the genesis timestamp, so the genesis header is unchanged
# and the validator's "7928 and 6780 before binaryTrieTime" holds.
spec() {
    jq '.config.cancunTime = 1000 | .config.amsterdamTime = 2000 | .config.binaryTrieTime = 1000000000' \
        /genesis.json > /pbt/genesis-pbt.json
}

# The merge plugin refuses to start without an engine port; both ports stay
# clear of the node hive booted.
config() { # snapshot path, preimages path
    jq -n --arg snap "$1" --arg pre "$2" '{
        Init: {ChainSpecPath: "/pbt/genesis-pbt.json", BaseDbPath: "/pbt/nm-db", DiscoveryEnabled: false,
               ProcessingEnabled: false, PeerManagerEnabled: false},
        JsonRpc: {Enabled: true, Host: "127.0.0.1", Port: 18545, EngineHost: "127.0.0.1", EnginePort: 18551},
        Network: {DiscoveryPort: 30399, P2PPort: 30399},
        FlatDb: {Enabled: true, Layout: "Flat", HistoryEnabled: false},
        Pbt: {Enabled: true, MigrationSnapshotPath: $snap, MigrationPreimagesPath: $pre, MigrationAnchor: 0}
    }' > /pbt/config.json
}

# boot runs the throwaway node. "anchor failed validation" is the importer's
# own rejection of a bad artifact; an import crash, a critical error, or any
# other exit is a crash, not a verdict. Success is the flat-state line logged
# after import, after which the node is killed rather than left to start
# networking.
boot() {
    local log=/pbt/nethermind.log
    rm -rf "$log" /pbt/nm-db
    "$NETHERMIND" --config /pbt/config.json --log INFO >"$log" 2>&1 &
    local pid=$!
    for _ in $(seq 1 90); do
        if grep -q "EIP-8347 migration: flat state at" "$log" 2>/dev/null; then
            kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
            echo "client_exit=0"
            return 0
        fi
        if grep -q "anchor failed validation" "$log" 2>/dev/null; then
            kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
            grep -a "anchor failed validation" "$log" >&2
            echo "client_exit=1"
            return 1
        fi
        if grep -qE "anchor import failed|A critical error has occurred" "$log" 2>/dev/null; then
            kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
            tail -20 "$log" >&2
            echo "client_exit=2"
            return 2
        fi
        if ! kill -0 "$pid" 2>/dev/null; then
            wait "$pid"; local status=$?
            echo "client_exit=$status"
            tail -20 "$log" >&2
            return 2
        fi
        sleep 1
    done
    kill "$pid" 2>/dev/null
    echo "neither finished nor failed within 90s" >&2
    return 2
}

# export writes both artifacts for block 0 from a throwaway preimage-flat node
# with $1 export workers, into /pbt/nm-out/export. A scheduled binaryTrieTime
# requires the Flat layout, so this boot reads /genesis.json unchanged.
export_artifacts() {
    rm -rf /pbt/nm-export-db /pbt/nm-out && mkdir -p /pbt/nm-out || return 2
    jq -n --argjson workers "$1" '{
        Init: {ChainSpecPath: "/genesis.json", BaseDbPath: "/pbt/nm-export-db", DiscoveryEnabled: false,
               PeerManagerEnabled: false},
        JsonRpc: {Enabled: true, Host: "127.0.0.1", Port: 18545, EngineHost: "127.0.0.1", EnginePort: 18551},
        Network: {DiscoveryPort: 30399, P2PPort: 30399},
        FlatDb: {Enabled: true, Layout: "PreimageFlat", HistoryEnabled: false},
        Pbt: {MigrationExportPath: "/pbt/nm-out/export", MigrationAnchor: 0, ExportConcurrency: $workers}
    }' > /pbt/nm-export.json || return 2
    timeout 120 "$NETHERMIND" --config /pbt/nm-export.json --log INFO >/pbt/nm-export.log 2>&1
    local status=$?
    [ $status -eq 124 ] && status=timeout
    echo "client_exit=$status"
    if [ "$status" != 0 ] || [ ! -f /pbt/nm-out/export/snapshot.pbt ] || [ ! -f /pbt/nm-out/export/preimages.bin ]; then
        tail -20 /pbt/nm-export.log >&2
        return 2
    fi
}

case "$verb" in
genesis-root)
    genesis_root
    ;;

verify)
    unpack || { echo "cannot unpack the fixtures" >&2; exit 2; }
    spec && config "$FIXTURES/$1" "$FIXTURES/$2" || { echo "cannot write the node's inputs" >&2; exit 2; }
    boot
    ;;

convert)
    if [ $# -gt 1 ]; then
        echo "plain-key state: no preimage store to remove from" >&2
        exit 3
    fi
    # The export is parallel, and the bytes must not depend on the workers.
    export_artifacts 4 || exit 2
    rm -rf /pbt/nm-four && mv /pbt/nm-out/export /pbt/nm-four || exit 2
    export_artifacts 1 || exit 2
    for f in snapshot.pbt preimages.bin; do
        if ! at=$(cmp /pbt/nm-out/export/$f /pbt/nm-four/$f); then
            echo "nondeterministic=$f: ${at:-one file is a prefix of the other}" >&2
            exit 2
        fi
    done
    echo "snapshot=$(b64 /pbt/nm-out/export/snapshot.pbt)"
    echo "preimages=$(b64 /pbt/nm-out/export/preimages.bin)"
    ;;

*)
    echo "unknown verb: $verb" >&2
    exit 2
    ;;
esac
