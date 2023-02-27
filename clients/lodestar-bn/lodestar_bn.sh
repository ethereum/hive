#!/bin/sh

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


# empty bootnodes file, required for custom testnet setup, use CLI arg instead to configure it.

echo "${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" > /data/testnet_setup/deposit_contract.txt

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
echo Container IP: $CONTAINER_IP
bootnodes_option=$([[ "$HIVE_ETH2_BOOTNODE_ENRS" == "" ]] && echo "" || echo "--bootnodes ${HIVE_ETH2_BOOTNODE_ENRS//,/ }")
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics.address=$CONTAINER_IP --metrics.port=$HIVE_ETH2_METRICS_PORT")
builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--builder --builder.urls $HIVE_ETH2_BUILDER_ENDPOINT")
echo BUILDER=$builder_option

echo "bootnodes option : ${bootnodes_option}"

echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret

echo Starting Lodestar Beacon Node

node /usr/app/node_modules/.bin/lodestar \
    beacon \
    --logLevel="$LOG" \
    --dataDir=/data/beacon \
    --port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --discoveryPort="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --paramsFile=/hive/input/config.yaml \
    --genesisStateFile=/hive/input/genesis.ssz \
    --rest \
    --rest.namespace="*" \
    --rest.address=0.0.0.0 \
    --rest.port="${HIVE_ETH2_BN_API_PORT:-4000}" \
    --eth1 \
    --execution.urls="$HIVE_ETH2_ETH1_ENGINE_RPC_ADDRS" \
    --eth1.depositContractDeployBlock=${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0} \
    --jwt-secret=/jwtsecret \
    $metrics_option \
    $bootnodes_option \
    $builder_option \
    --enr.ip="${CONTAINER_IP}" \
    --enr.tcp="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --enr.udp="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --network.connectToDiscv5Bootnodes=true \
    --discv5 \
    --subscribeAllSubnets=true \
    --targetPeers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}"
