#!/bin/bash

if test -f "/hive/input/genesis.ssz"; then
    if [ -z "$HIVE_ETH2_ETH1_RPC_ADDRS" ]; then
      echo "genesis.ssz file is missing, and no Eth1 RPC addr was provided for building genesis from scratch."
      # TODO: alternative to start from weak-subjectivity-state
      exit 1
    fi
fi

mkdir -p /data/testnet_setup

cp /hive/input/genesis.ssz /data/testnet_setup/genesis.ssz

# empty bootnodes file, required for custom testnet setup, use CLI arg instead to configure it.
echo "[]" > /data/testnet_setup/boot_enr.yaml

echo "${DEPOSIT_CONTRACT_ADDRESS:-'0x1111111111111111111111111111111111111111'}" > /data/testnet_setup/deposit_contract.txt
echo "${HIVE_ETH2_DEPOSIT_DEPLOY_BLOCK_NUMBER:-'0'}" > /data/testnet_setup/deploy_block.txt

/make_config.sh > /data/testnet_setup/config.yaml

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

eth1_option=$([[ "$ETH1_RPC_ADDRS" == "" ]] && echo "--dummy-eth1" || echo "--eth1-endpoints=$ETH1_RPC_ADDRS")
ip_option=$([[ "$HIVE_ETH2_P2P_IP_ADDRESS" == "" ]] && echo "" || echo "--disable-enr-auto-update=true  --enr-address='$HIVE_ETH2_P2P_IP_ADDRESS'")
metrics_option=$([[ "$HIVE_ETH2_METRICS_PORT" == "" ]] && echo "" || echo "--metrics --metrics-address=0.0.0.0 --metrics-port='$HIVE_ETH2_METRICS_PORT' --metrics-allow-origin='*")

lighthouse \
    --debug-level="$LOG" \
    --datadir=/data/beacon \
    --testnet-dir=/data/testnet_setup \
    bn \
    --network-dir=/data/network \
    $ip_option $metrics_option $eth1_option \
    --enr-tcp-port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --enr-udp-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}" \
    --port="${HIVE_ETH2_P2P_TCP_PORT:-9000}" \
    --discovery-port="${HIVE_ETH2_P2P_UDP_PORT:-9000}"  \
    --target-peers="${HIVE_ETH2_P2P_TARGET_PEERS:-10}" \
    --boot-nodes="${HIVE_ETH2_BOOTNODE_ENRS:-""}" \
    --max-skip-slots="${HIVE_ETH2_MAX_SKIP_SLOTS:-1000}" \
    --http --http-address=0.0.0.0 --http-port="${HIVE_ETH2_BN_API_PORT:-4000}" --http-allow-origin="*"
