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

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    sed -i '/TERMINAL_TOTAL_DIFFICULTY/d' /data/testnet_setup/config.yaml
    echo "TERMINAL_TOTAL_DIFFICULTY: $HIVE_TERMINAL_TOTAL_DIFFICULTY" >> /data/testnet_setup/config.yaml
fi
if [[ "$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" != "" ]]; then
    echo "SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY: $HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" >> /data/testnet_setup/config.yaml
fi

echo config.yaml:
cat /data/testnet_setup/config.yaml

# empty bootnodes file, required for custom testnet setup, use CLI arg instead to configure it.
echo "[]" > /data/testnet_setup/boot_enr.yaml

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" > /data/testnet_setup/deploy_block.txt

mkdir -p /data/beacon
mkdir -p /data/network

# Set static private keys to be able to always set nodes as trusted peers
trustedpeers="16Uiu2HAmKJJED6835NsYwwT3MZVVi4idg2jiULBYb1kPzqw9jzAM,16Uiu2HAm9XVoQGJGQJ9SAbMuGcCFG81Ch2EGCukmAix8g5yw9mcp,16Uiu2HAm3CnjeveoribYTQiWjkxUq2R6QrQwvZsyAzm75UXTL8aL,16Uiu2HAm96eiZf7YktvxfJ3AeXyrRNq4Cqf2Ypda6Y4nL17xwsXX,16Uiu2HAmNWiGjii9thAP6dzMcRxwQF317NraEri6qnYswvFVP1Mg"

pks=("fd5fd778baa59f457bd671d61839f6dbf8f4ef4d4df67d621598a60ff212f07c" "b97bb33696dfb44e9bf3376b4247753ae4e55ba7b90b26153e0f40a00e63fc2f" "822c4f5856e7a5a7f7c1d4ca4d262df368f7d1323225bbe2c7c015401e422be5" "b09f940d452b33069aba7d3b7fc21725b5a0f6a0c2b2bb7eb6954c6c3295dfdb" "f87b71646c9850e5f7e4976c388d183832d6408491788afb5edba467617a9bd6")

echo "beacon index: $HIVE_ETH2_BEACON_NODE_INDEX"
if [[ "$HIVE_ETH2_BEACON_NODE_INDEX" != "" ]]; then
    pk="${pks[HIVE_ETH2_BEACON_NODE_INDEX]}"
    if [[ "$pk" != "" ]]; then
        key="$(echo $pk | xxd -r -p)"
        echo "networking pk: $pk"
        echo -n $key > /data/network/key
    fi
fi

if [[ "$HIVE_ETH2_TRUSTED_PEER_IDS" != "" ]]; then
    trustedpeers="$trustedpeers,$HIVE_ETH2_TRUSTED_PEER_IDS"
fi

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
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics-address=0.0.0.0 --metrics-port=$HIVE_ETH2_METRICS_PORT --metrics-allow-origin=*")
if [ "$HIVE_ETH2_MERGE_ENABLED" != "" ]; then
    echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret
    merge_option="--execution-endpoints=$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS --jwt-secrets=/jwtsecret"
fi
opt_sync_option=$([[ "$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" == "" ]] && echo "" || echo "--safe-slots-to-import-optimistically=$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY")
builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--builder=$HIVE_ETH2_BUILDER_ENDPOINT")
echo BUILDER=$builder_option

lighthouse \
    --debug-level="$LOG" \
    --datadir=/data/beacon \
    --testnet-dir=/data/testnet_setup \
    bn \
    --network-dir=/data/network \
    $metrics_option $builder_option $merge_option $opt_sync_option \
    --enr-tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --enr-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --enr-address="${CONTAINER_IP}" \
    --disable-enr-auto-update=true  \
    --port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --discovery-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}"  \
    --target-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
    --trusted-peers="$trustedpeers" \
    --subscribe-all-subnets \
    --boot-nodes="${HIVE_ETH2_BOOTNODE_ENRS:-""}" \
    --max-skip-slots="${HIVE_ETH2_MAX_SKIP_SLOTS:-1000}" \
    --http --http-address=0.0.0.0 --http-port="${HIVE_ETH2_BN_API_PORT:-4000}" --http-allow-origin="*"
