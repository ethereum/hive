#!/usr/bin/env bash


echo "Starting simulator."
cd /go/src/github.com/ethereum/hive/simulators/ethereum/graphql/

echo "Simulator endpoint: $HIVE_SIMULATOR"
echo "Simulator parallelism: $HIVE_PARALLELISM"

echo "{ \"hostURI\":\"$HIVE_SIMULATOR\" }" >hiveProviderConfig.json

go install 


if [ "${HIVE_DEBUG}" = true ]; then
    dlv debug  	--headless  --listen=:2345 --log=true  --api-version=2 -- -simProvider="hive" -providerConfig="hiveProviderConfig.json"  
else
   
   /go/bin/graphql -simProvider="hive" -providerConfig="hiveProviderConfig.json"
fi



echo "Ending simulator."

