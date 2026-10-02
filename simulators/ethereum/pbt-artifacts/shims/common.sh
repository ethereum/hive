# Sourced by every shim. Exit codes: 0 accepted, 1 rejected, 3 unsupported,
# anything else a crash. Never wrap the client in `|| exit 1`.

FIXTURES=/pbt/fixtures

unpack() {
    [ -d "$FIXTURES" ] && return 0
    mkdir -p "$FIXTURES" && tar -xf /pbt-fixtures.tar -C "$FIXTURES" 2>/dev/null || { rm -rf "$FIXTURES"; return 1; }
}

genesis_root() {
    root=$(curl -sf -X POST -H 'Content-Type: application/json' \
        --data '{"jsonrpc":"2.0","id":1,"method":"eth_getBlockByNumber","params":["0x0",false]}' \
        http://127.0.0.1:8545 | jq -er '.result.stateRoot') || { echo "no answer from the node's RPC" >&2; exit 2; }
    echo "mpt_root=$root"
}

b64() { base64 -w0 "$1" 2>/dev/null || base64 "$1" | tr -d '\n'; }
