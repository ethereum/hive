#!/bin/bash

# Immediately abort the script on any error encountered
set -e

if [ ! -f "/hive/input/genesis.ssz" ]; then
    if [ -z "$HIVE_ETH2_ETH1_RPC_ADDRS" ]; then
      echo "genesis.ssz file is missing, and no Eth1 RPC addr was provided for building genesis from scratch."
      exit 1
    fi
fi

mkdir -p /data/testnet_setup

cp /hive/input/genesis.ssz /data/testnet_setup/genesis.ssz
cp /hive/input/config.yaml /data/testnet_setup

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    sed -i '/TERMINAL_TOTAL_DIFFICULTY/d' /data/testnet_setup/config.yaml
    echo "TERMINAL_TOTAL_DIFFICULTY: $HIVE_TERMINAL_TOTAL_DIFFICULTY" >> /data/testnet_setup/config.yaml
fi
if [[ "$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" != "" ]]; then
    echo "SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY: $HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" >> /data/testnet_setup/config.yaml
fi

echo config.yaml:
cat /data/testnet_setup/config.yaml

mkdir -p /data/teku

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0|1) LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

echo "bootnodes: ${HIVE_ETH2_BOOTNODE_ENRS}"
echo "staticpeers: ${HIVE_ETH2_STATIC_PEERS}"

CONTAINER_IP=`hostname -i | awk '{print $1;}'`
eth1_option=$([[ "$HIVE_ETH2_ETH1_RPC_ADDRS" == "" ]] && echo "" || echo "--eth1-endpoints=$HIVE_ETH2_ETH1_RPC_ADDRS")
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics-enabled=true --metrics-interface=0.0.0.0 --metrics-port=$HIVE_ETH2_METRICS_PORT --metrics-host-allowlist=*")
enr_option=$([[ "$HIVE_ETH2_BOOTNODE_ENRS" == "" ]] && echo "" || echo --p2p-discovery-bootnodes="$HIVE_ETH2_BOOTNODE_ENRS")
static_option=$([[ "$HIVE_ETH2_STATIC_PEERS" == "" ]] && echo "" || echo --p2p-static-peers="$HIVE_ETH2_STATIC_PEERS")
opt_sync_option=$([[ "$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" == "" ]] && echo "" || echo "--Xnetwork-safe-slots-to-import-optimistically=$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY")
builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--validators-builder-registration-default-enabled=true --validators-proposer-blinded-blocks-enabled=true --builder-endpoint=$HIVE_ETH2_BUILDER_ENDPOINT")
echo $builder_option

if [ "$HIVE_ETH2_MERGE_ENABLED" != "" ]; then
    echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret
    merge_option="--ee-endpoint=$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS --ee-jwt-secret-file=/jwtsecret"
fi

echo Starting Teku Beacon Node

/opt/teku/bin/teku \
    --network=/data/testnet_setup/config.yaml \
    --data-path=/data/teku \
    --initial-state=/data/testnet_setup/genesis.ssz \
    --eth1-deposit-contract-address="${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" \
    --log-destination console \
    --logging="$LOG" \
    $metrics_option $eth1_option $merge_option $enr_option $static_option $opt_sync_option $builder_option \
    --validators-proposer-default-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b" \
    --p2p-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --p2p-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --p2p-advertised-ip="${CONTAINER_IP}" \
    --p2p-peer-lower-bound="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
    --rest-api-enabled=true --rest-api-interface=0.0.0.0 --rest-api-port="${HIVE_ETH2_BN_API_PORT:-4000}" --rest-api-host-allowlist="*" \
    --data-storage-mode=ARCHIVE \
    --Xstartup-target-peer-count=0 \
    --p2p-subscribe-all-subnets-enabled
