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

mkdir -p /data/beacon
mkdir -p /data/network

LOG=info
case "$HIVE_LOGLEVEL" in
    0)   LOG=fatal ;;
    1)   LOG=error ;;
    2)   LOG=warn  ;;
    3)   LOG=info  ;;
    4)   LOG=debug ;;
    5)   LOG=trace ;;
esac

echo "bootnodes: ${HIVE_ETH2_BOOTNODE_ENRS}"

CONTAINER_IP=`hostname -i | awk '{print $1;}'`
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "--disable-monitoring=true" || echo "--disable-monitoring=false --monitoring-host=0.0.0.0 --monitoring-port=$HIVE_ETH2_METRICS_PORT")
echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret


/app/cmd/beacon-chain/beacon-chain \
    --verbosity="$LOG" \
    --accept-terms-of-use=true \
    --datadir=/data/beacon \
    --chain-config-file=/hive/input/config.yaml \
    --genesis-state=/hive/input/genesis.ssz \
    --bootstrap-node="${HIVE_ETH2_BOOTNODE_ENRS:-""}" \
    --p2p-tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --p2p-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --p2p-host-ip="${CONTAINER_IP}" \
    --p2p-local-ip="${CONTAINER_IP}" \
    --http-web3provider="$HIVE_ETH2_ETH1_RPC_ADDRS" \
    --jwt-secret=/jwtsecret \
    --min-sync-peers=1 \
    --subscribe-all-subnets=true \
    $metrics_option \
    --deposit-contract="${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" \
    --contract-deployment-block="${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" \
    --rpc-host=0.0.0.0 --rpc-port="${HIVE_ETH2_BN_API_PORT:-4000}" \
    --grpc-gateway-host=0.0.0.0 --grpc-gateway-port="${HIVE_ETH2_BN_GATEWAY_PORT:-3500}" \
    --suggested-fee-recipient="0xa94f5374fce5edbc8e2a8697c15331677e6ebf0b"

# --interop-genesis-state=/hive/input/genesis.ssz \
# --max-skip-slots="${HIVE_ETH2_MAX_SKIP_SLOTS:-1000}"
# --target-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}"
# --disable-enr-auto-update=true 
# --network-dir=/data/network
