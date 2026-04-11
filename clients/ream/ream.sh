#!/bin/bash

set -euo pipefail

DEVNET_LABEL="${HIVE_LEAN_DEVNET_LABEL:-devnet3}"
NODE_ID="${HIVE_NODE_ID:-ream_0}"
BOOTNODES="${HIVE_BOOTNODES:-none}"
GENESIS_VALIDATORS_OVERRIDE="${HIVE_LEAN_GENESIS_VALIDATORS:-}"
GENESIS_VALIDATOR_ENTRIES_OVERRIDE="${HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES:-}"
KEY_FILE=""
NETWORK_CONFIG="${HIVE_LEAN_NETWORK_CONFIG:-ephemery}"

case "$DEVNET_LABEL" in
    devnet3)
        DEFAULT_REAM_BIN="/usr/local/bin/ream-devnet3"
        ;;
    devnet4)
        DEFAULT_REAM_BIN="/usr/local/bin/ream-devnet4"
        ;;
    *)
        echo "Unsupported Lean devnet label: $DEVNET_LABEL" >&2
        exit 1
        ;;
esac

REAM_BIN="${REAM_BIN:-$DEFAULT_REAM_BIN}"

cleanup() {
    if [ -n "$KEY_FILE" ] && [ -f "$KEY_FILE" ]; then
        rm -f "$KEY_FILE"
    fi
}

trap cleanup EXIT

default_genesis_validators() {
    case "$DEVNET_LABEL" in
        devnet3|devnet4)
            cat <<'EOF'
0x4ebeee6aa607247dadcba631f8ffb473e170c9000181261de029a149a86d825c60c3f626315c9b549d425f2f6d51d94bcbe14a02
0x7017345bbe0feb6b7a926e1f0b988158b7552e81fbbaebd43a87a0cb679f247987f275ba0ca85df1131669f210b5e777a3797618
0x1859df2e6b17775d054a696990aa680b5dcfdfcd5c063b9c6b7f90ed4da48d2a8a503f73574ddde2b0409e148dc42129980ba1af
EOF
            ;;
        *)
            return 1
            ;;
    esac
}

if [ -n "${HIVE_LEAN_GENESIS_TIME:-}" ]; then
    mkdir -p /tmp/ream-network
    if [ -n "$GENESIS_VALIDATOR_ENTRIES_OVERRIDE" ]; then
        IFS=',' read -r -a GENESIS_VALIDATOR_ENTRIES <<< "$GENESIS_VALIDATOR_ENTRIES_OVERRIDE"
    elif [ -n "$GENESIS_VALIDATORS_OVERRIDE" ]; then
        IFS=',' read -r -a GENESIS_VALIDATORS <<< "$GENESIS_VALIDATORS_OVERRIDE"
    else
        if [ "$DEVNET_LABEL" = "devnet4" ]; then
            echo "devnet4 custom genesis startup requires HIVE_LEAN_GENESIS_VALIDATOR_ENTRIES" >&2
            exit 1
        fi

        mapfile -t GENESIS_VALIDATORS < <(default_genesis_validators)
    fi

    if [ -n "$GENESIS_VALIDATOR_ENTRIES_OVERRIDE" ]; then
        GENESIS_VALIDATOR_COUNT="${#GENESIS_VALIDATOR_ENTRIES[@]}"
    else
        GENESIS_VALIDATOR_COUNT="${#GENESIS_VALIDATORS[@]}"
    fi

    cat > /tmp/ream-network/config.yaml <<EOF
GENESIS_TIME: ${HIVE_LEAN_GENESIS_TIME}
NUM_VALIDATORS: ${GENESIS_VALIDATOR_COUNT}
GENESIS_VALIDATORS:
EOF

    if [ -n "$GENESIS_VALIDATOR_ENTRIES_OVERRIDE" ]; then
        for validator_entry in "${GENESIS_VALIDATOR_ENTRIES[@]}"; do
            IFS='|' read -r attestation_public_key proposal_public_key <<< "$validator_entry"
            printf '  - attestation_public_key: "%s"\n' "$attestation_public_key" >> /tmp/ream-network/config.yaml
            printf '    proposal_public_key: "%s"\n' "$proposal_public_key" >> /tmp/ream-network/config.yaml
        done
    else
        for validator in "${GENESIS_VALIDATORS[@]}"; do
            printf '  - "%s"\n' "$validator" >> /tmp/ream-network/config.yaml
        done
    fi

    NETWORK_CONFIG="/tmp/ream-network/config.yaml"
fi

FLAGS=(
    --network "$NETWORK_CONFIG"
    --validator-registry-path /assets/lean/validators.yaml
    --node-id "$NODE_ID"
    --socket-address 0.0.0.0
    --socket-port 9000
    --http-address 0.0.0.0
    --http-port 5052
    --bootnodes "$BOOTNODES"
)

if [ -n "${HIVE_CHECKPOINT_SYNC_URL:-}" ]; then
    FLAGS+=(--checkpoint-sync-url "$HIVE_CHECKPOINT_SYNC_URL")
fi

if [ "${HIVE_IS_AGGREGATOR:-0}" = "1" ]; then
    FLAGS+=(--is-aggregator)
fi

if [ -n "${HIVE_CLIENT_PRIVATE_KEY:-}" ]; then
    KEY_FILE="$(mktemp /tmp/ream-node-key.XXXXXX)"
    printf "%s" "$HIVE_CLIENT_PRIVATE_KEY" > "$KEY_FILE"
    FLAGS+=(--private-key-path "$KEY_FILE")
fi

if [ "${HIVE_METRICS_ENABLED:-0}" = "1" ]; then
    FLAGS+=(--metrics --metrics-address 0.0.0.0)
fi

export RUST_LOG="${RUST_LOG:-info}"

exec "$REAM_BIN" --ephemeral lean_node "${FLAGS[@]}"
