#!/bin/bash

# Immediately abort the script on any error encountered
set -e

if [ ! -f "/hive/input/genesis.ssz" ]; then
    if [ -z "$HIVE_ETH2_ETH1_RPC_ADDRS" ]; then
      echo "genesis.ssz file is missing, and no Eth1 RPC addr was provided for building genesis from scratch."
      # TODO: alternative to start from weak-subjectivity-state
      exit 1
    fi
fi

mkdir -p /data/testnet_setup

cp /hive/input/genesis.ssz /data/testnet_setup/genesis.ssz
cp /hive/input/config.yaml /data/testnet_setup

# empty bootnodes file, required for custom testnet setup, use CLI arg instead to configure it.
echo "[]" > /data/testnet_setup/boot_enr.yaml

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" > /data/testnet_setup/deploy_block.txt

mkdir -p /data/beacon
mkdir -p /data/network

LOG=info
case "$HIVE_LOGLEVEL" in
    0|1) LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

echo "bootnodes: ${HIVE_ETH2_BOOTNODE_ENRS}"

CONTAINER_IP=`hostname -i | awk '{print $1;}'`
eth1_option=$([[ "$HIVE_ETH2_ETH1_RPC_ADDRS" == "" ]] && echo "--dummy-eth1" || echo "--eth1-endpoints=$HIVE_ETH2_ETH1_RPC_ADDRS")
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics-address=0.0.0.0 --metrics-port=$HIVE_ETH2_METRICS_PORT --metrics-allow-origin=*")
if [ "$HIVE_ETH2_MERGE_ENABLED" != "" ]; then
    echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret
    merge_option="--merge --execution-endpoints=$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS --jwt-secrets=/jwtsecret"
fi

lighthouse \
    --debug-level="$LOG" \
    --datadir=/data/beacon \
    --testnet-dir=/data/testnet_setup \
    bn \
    --network-dir=/data/network \
    $metrics_option $eth1_option $merge_option \
    --enr-tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --enr-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --enr-address="${CONTAINER_IP}" \
    --disable-enr-auto-update=true  \
    --port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --discovery-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}"  \
    --target-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
    --boot-nodes="${HIVE_ETH2_BOOTNODE_ENRS:-""}" \
    --max-skip-slots="${HIVE_ETH2_MAX_SKIP_SLOTS:-1000}" \
    --http --http-address=0.0.0.0 --http-port="${HIVE_ETH2_BN_API_PORT:-4000}" --http-allow-origin="*"
