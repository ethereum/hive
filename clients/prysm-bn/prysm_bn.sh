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

# znrt encodes the values of these items between double quotes (""), which is against the spec:
# https://github.com/ethereum/consensus-specs/blob/v1.1.10/configs/mainnet.yaml
sed -i 's/"\([[:digit:]]\+\)"/\1/' /hive/input/config.yaml
sed -i 's/"\(0x[[:xdigit:]]\+\)"/\1/' /hive/input/config.yaml

if [ "$HIVE_TERMINAL_TOTAL_DIFFICULTY" != "" ]; then
    sed -i '/TERMINAL_TOTAL_DIFFICULTY/d' /hive/input/config.yaml
    echo "TERMINAL_TOTAL_DIFFICULTY: $HIVE_TERMINAL_TOTAL_DIFFICULTY" >> /hive/input/config.yaml
fi

if [[ "$HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" != "" ]]; then
    echo "SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY: $HIVE_ETH2_SAFE_SLOTS_TO_IMPORT_OPTIMISTICALLY" >> /hive/input/config.yaml
fi

echo config.yaml:
cat /hive/input/config.yaml

CONTAINER_IP=`hostname -i | awk '{print $1;}'`
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "--disable-monitoring=true" || echo "--disable-monitoring=false --monitoring-host=0.0.0.0 --monitoring-port=$HIVE_ETH2_METRICS_PORT")
echo -n "0x7365637265747365637265747365637265747365637265747365637265747365" > /jwtsecret

if [[ "$HIVE_ETH2_BOOTNODE_ENRS" == "" ]]; then
    bootnode_option=""
else
    bootnode_option=""
    for bn in ${HIVE_ETH2_BOOTNODE_ENRS//,/ }; do
        trimmed_bn=${bn//=/}
        bootnode_option="$bootnode_option --bootstrap-node=$trimmed_bn"
    done
fi
builder_option=$([[ "$HIVE_ETH2_BUILDER_ENDPOINT" == "" ]] && echo "" || echo "--http-mev-relay=$HIVE_ETH2_BUILDER_ENDPOINT")
echo BUILDER=$builder_option

echo Starting Prysm Beacon Node

/beacon-chain \
    --verbosity="$LOG" \
    --accept-terms-of-use=true \
    --datadir=/data/beacon \
    --chain-config-file=/hive/input/config.yaml \
    --genesis-state=/hive/input/genesis.ssz \
    $bootnode_option \
    --p2p-tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --p2p-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --p2p-host-ip="${CONTAINER_IP}" \
    --p2p-local-ip="${CONTAINER_IP}" \
    --execution-endpoint="$HIVE_ETH2_ETH1_RPC_ADDRS" \
    --jwt-secret=/jwtsecret \
    --min-sync-peers=1 \
    --subscribe-all-subnets=true \
    --enable-debug-rpc-endpoints=true \
    $metrics_option \
    $builder_option \
    --deposit-contract="${HIVE_ETH2_CONFIG_DEPOSIT_CONTRACT_ADDRESS:-0x1111111111111111111111111111111111111111}" \
    --contract-deployment-block="${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-0}" \
    --rpc-host=0.0.0.0 --rpc-port="${HIVE_ETH2_BN_GRPC_PORT:-3500}" \
    --grpc-gateway-host=0.0.0.0 --grpc-gateway-port="${HIVE_ETH2_BN_API_PORT:-4000}" --grpc-gateway-corsdomain="*" \
    --suggested-fee-recipient="0xa94f5374Fce5edBC8E2a8697C15331677e6EbF0B"
# NOTE: gRPC/RPC ports are inverted to allow the simulator to access the REST API