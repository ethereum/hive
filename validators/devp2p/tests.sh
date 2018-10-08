#!/usr/bin/env bash

# Execute RPC test suite
echo "Run Devp2p tests against TODO"
/devp2p.test -test.v -test.run Discovery $"${ips[@]}"

#echo "Run RPC-WEBSOCKET tests..."
#/rpc.test -test.run Eth/websocket $"${ips[@]}"

# Run hive (must be run from root directory of hive repo)
# hive --client=go-ethereum:master --test=NONE --sim=ethereum/rpc/eth --docker-noshell -loglevel 6
#
# Logs can be found in workspace/logs/simulations/ethereum/rpc\:eth\[go-ethereum\:develop\]/simulator.log
