#!/bin/bash
# Returns the enode URL for this Fukuii instance.
# Called by hive to discover the node's P2P identity.
set -uo pipefail

# admin_nodeInfo is the authoritative source. The `admin` namespace must be
# exposed for this to work — see the rpc.apis line in fukuii.sh.
RESULT=$(curl -s -X POST -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"admin_nodeInfo","params":[],"id":1}' \
  http://localhost:8545 2>/dev/null)

ENODE=$(echo "$RESULT" | jq -r '.result.enode // empty' 2>/dev/null)

if [ -n "$ENODE" ]; then
    echo "$ENODE"
    exit 0
fi

# Fallback: parse from the server log. Only works when the log level lets the
# startup enode line through; hive runs at ERROR, so this is best-effort.
ENODE=$(grep -ho 'enode://[^ ]*' /app/data/logs/*.log 2>/dev/null | head -1)
if [ -n "$ENODE" ]; then
    echo "$ENODE"
    exit 0
fi

# No enode could be determined. Fail loudly.
#
# This previously printed `enode://unknown@127.0.0.1:30303`. That is not a
# usable enode — hive hex-decodes the public key and chokes on the 'u' of
# "unknown", failing every devp2p test with
# `invalid public key (encoding/hex: invalid byte: U+0075 'u')`, which says
# nothing about the actual problem. A placeholder that cannot possibly work
# turns a clear failure ("the node did not report an enode") into a confusing
# one, and hid a one-line configuration gap for months.
{
    echo "enode.sh: could not determine this node's enode URL."
    echo "  admin_nodeInfo returned: ${RESULT:-<no response>}"
    echo "  Check that the 'admin' namespace is in fukuii.network.rpc.apis and"
    echo "  that the JSON-RPC server is up on :8545."
} >&2
exit 1
