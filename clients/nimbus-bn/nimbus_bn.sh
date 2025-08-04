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

cp /hive/input/genesis.ssz /data/testnet_setup
cp /hive/input/config.yaml /data/testnet_setup

# znrt encodes the values of these items between double quotes (""), which is against the spec:
# https://github.com/ethereum/consensus-specs/blob/v1.1.10/configs/mainnet.yaml
# Nimbus is the only client that complains about this.
sed -i 's/"//g' /data/testnet_setup/config.yaml
sed -i '/TERMINAL_BLOCK_HASH_ACTIVATION_EPOCH/d' /data/testnet_setup/config.yaml
echo Using config.yaml
cat /data/testnet_setup/config.yaml
echo ""

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" > /data/testnet_setup/deposit_contract_block.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_HASH:-0}" > /data/testnet_setup/deposit_contract_block_hash.txt

mkdir -p /data/beacon
chmod 0700 /data/beacon

LOG=INFO
case "$HIVE_LOGLEVEL" in
    0|1) LOG=ERROR ;;
    2)   LOG=WARN  ;;
    3)   LOG=INFO  ;;
    4)   LOG=DEBUG ;;
    5)   LOG=TRACE ;;
esac

echo "bootnodes: ${HIVE_ETH2_BOOTNODE_ENRS}"

CONTAINER_IP=`hostname -i | awk '{print $1;}'`
echo Container IP: "${CONTAINER_IP}"

if [[ "$HIVE_ETH2_BOOTNODE_ENRS" == "" ]]; then
    bootnodes_option="--subscribe-all-subnets"
else
    bootnodes_option=""
    for bn in ${HIVE_ETH2_BOOTNODE_ENRS//,/ }; do
        bootnodes_option="$bootnodes_option --bootstrap-node=$bn"
    done
fi
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics-address=0.0.0.0 --metrics-port=$HIVE_ETH2_METRICS_PORT")

builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--payload-builder=true --payload-builder-url=$HIVE_ETH2_BUILDER_ENDPOINT")
echo BUILDER=$builder_option

echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret

echo Starting Nimbus Beacon Node

/usr/bin/nimbus_beacon_node \
    --non-interactive=true \
    --log-level="$LOG" \
    --data-dir=/data/beacon \
    --network=/data/testnet_setup \
    --web3-url="$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS" \
    --jwt-secret=/jwtsecret \
    --num-threads=4 \
    $bootnodes_option $metrics_option $builder_option \
    --nat="extip:${CONTAINER_IP}" \
    --listen-address=0.0.0.0 \
    --tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --rest --rest-address=0.0.0.0 --rest-port="${HIVE_ETH2_BN_API_PORT:-4000}" --rest-allow-origin="*" \
    --doppelganger-detection=false \
    --max-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
    --enr-auto-update=false
