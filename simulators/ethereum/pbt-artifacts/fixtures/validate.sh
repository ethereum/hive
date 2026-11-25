#!/usr/bin/env bash
# The simulator's judgement, against one geth binary and no docker. It
# generates the fixture set the way the image does, then the valid pair must
# verify and every reject case must be refused. A producer case must make the
# converter refuse its source for a missing preimage. With an execution-specs
# checkout, generating also holds the pair to the spec reference's root.
#
# Usage: ./validate.sh /path/to/geth [/path/to/execution-specs]
set -u

geth="${1:-geth}"
case $geth in /*) ;; */*) geth="$PWD/$geth" ;; esac
specs="${2:-}"
here="$(cd "$(dirname "$0")" && pwd)"
fx="$(mktemp -d)"
datadir="$(mktemp -d)"
trap 'rm -rf "$fx" "$datadir"' EXIT

(
    cd "$here/gen" || exit 2
    [ -f go.mod ] || { cp go.mod.dist go.mod && cp go.sum.dist go.sum; } || exit 2
    go run -tags pbtgen . -geth "$geth" -out "$fx" ${specs:+-ref "$specs"}
) || { echo "FATAL: the fixture set does not generate"; exit 2; }

"$geth" --datadir "$datadir" init "$fx/genesis.json" >/dev/null 2>&1 || { echo "FATAL: genesis init failed"; exit 2; }

verify() { # snapshot preimages -> exit status
    "$geth" --datadir "$datadir" bintrie import --verify-only "$fx/$1" "$fx/$2" 0 >/dev/null 2>&1 </dev/null
}

# produce converts a fresh source after applying the case's defect: each
# drop-preimage <hash> deletes the store key "secure-key-" + hash.
produce() {
    local dir status key out
    dir="$(mktemp -d)"
    "$geth" --datadir "$dir" --cache.preimages init "$fx/genesis.json" >/dev/null 2>&1 || { rm -rf "$dir"; return 2; }
    while [ $# -ge 2 ]; do
        key=0x7365637572652d6b65792d${2#0x}
        { "$geth" --datadir "$dir" db get "$key" && "$geth" --datadir "$dir" db delete "$key"; } >/dev/null 2>&1 || { rm -rf "$dir"; return 2; }
        shift 2
    done
    out=$("$geth" --datadir "$dir" bintrie convert --snapshot-out "$dir/s.bin" --preimages-out "$dir/p.bin" 2>&1 </dev/null)
    status=$?
    if [ $status -eq 0 ]; then
        rm -rf "$dir"
        return 0
    fi
    if [ -f "$dir/s.bin" ] || [ -f "$dir/p.bin" ]; then
        echo "$out" >&2
        echo "rejected, yet printed an artifact" >&2
        rm -rf "$dir"
        return 2
    fi
    if echo "$out" | grep -qE 'missing preimage for (account hash|flat account|storage key|flat storage key)'; then
        rm -rf "$dir"
        return 1
    fi
    echo "$out" >&2
    rm -rf "$dir"
    return 2
}

pass=0 fail=0
verify valid/snapshot.bin valid/preimages.bin
status=$?
case $status in
0) pass=$((pass + 1)) ;;
1) fail=$((fail + 1)); echo "FAIL valid pair: rejected" ;;
*) fail=$((fail + 1)); echo "FAIL valid pair: crashed (exit $status)" ;;
esac

while IFS=$'\t' read -r id snapshot preimages; do
    verify "$snapshot" "$preimages"
    status=$?
    case $status in
    0) fail=$((fail + 1)); echo "FAIL $id: accepted" ;;
    1) pass=$((pass + 1)) ;;
    *) fail=$((fail + 1)); echo "FAIL $id: crashed (exit $status)" ;;
    esac
done < <(jq -r '.cases[] | select(.suite != "produce") | [.id, .snapshot, .preimages] | @tsv' "$fx/manifest.json")

while IFS=$'\t' read -r id defect; do
    # shellcheck disable=SC2086 # the defect is an argument list
    produce $defect
    case $? in
    1) pass=$((pass + 1)) ;;
    0) fail=$((fail + 1)); echo "FAIL $id: converted" ;;
    *) fail=$((fail + 1)); echo "FAIL $id: the converter or the defect crashed" ;;
    esac
done < <(jq -r '.cases[] | select(.suite == "produce") | [.id, (.defect | join(" "))] | @tsv' "$fx/manifest.json")

echo "--- $pass passed, $fail failed"
[ "$fail" -eq 0 ]
