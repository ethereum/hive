#!/usr/bin/env bash


echo "Starting simulator."
cd /go/src/github.com/ethereum/hive/simulators/devp2p/basic/

echo "Simulator endpoint: $HIVE_SIMULATOR"



if [ "${HIVE_DEBUG}" = true ]; then
    dlv test  	--headless  --listen=:2345 --log=true  --api-version=2 -- -simulatorHost "$HIVE_SIMULATOR" -test.parallel "$HIVE_PARALLELISM" -test.v
else
   
    go test -simulatorHost "$HIVE_SIMULATOR"  -test.parallel $HIVE_PARALLELISM  -test.v
fi



echo "Ending simulator."
#/devp2p.test -test.v -test.run Discovery/discoveryv4/v4001  -simulatorHost "$HIVE_SIMULATOR" 


# Run hive (must be run from root directory of hive repo)
# hive --client=go-ethereum:master --test=NONE --sim=ethereum/rpc/eth --docker-noshell -loglevel 6
#
# Logs can be found in workspace/logs/simulations/ethereum/rpc\:eth\[go-ethereum\:develop\]/simulator.log
