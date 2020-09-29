#!/usr/bin/env bash


echo "Starting simulator."
cd /go/src/github.com/ethereum/hive/simulators/ethereum/sync/

echo "Simulator endpoint: $HIVE_SIMULATOR"
echo "Simulator parallelism: $HIVE_PARALLELISM"


if [ "${HIVE_DEBUG}" = true ]; then
    dlv test  	--headless  --listen=:2345 --log=true  --api-version=2  -test.parallel "$HIVE_PARALLELISM" -test.v
else
    go test -test.parallel $HIVE_PARALLELISM  -test.v -test.timeout 0
fi



echo "Ending simulator."

