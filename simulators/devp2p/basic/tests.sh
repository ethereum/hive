#!/usr/bin/env bash

# Execute RPC test suite
#TODO Get ENODE ID from other client

#TARGET_ENODE=enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@$HIVE_CLIENT_IP:30303

#TARGET_ENODE is defined in the following script, which is obtained
#from the client container during simulator or validator execution. It is client-specific.
echo "Starting simulator."
cd /tests/simulators/devp2p/basic/
#dlv exec /go/src/github.com/frankszendzielarz/hive/simulators/devp2p/basic/devp2p.test --headless  --listen=:2345 --log   --api-version=2 -- -simulatorHost "$HIVE_SIMULATOR" 

dlv test  --headless  --listen=:2345 --log=true  --api-version=1 -- -simulatorHost "$HIVE_SIMULATOR" 

echo "Ending simulator."
#/devp2p.test -test.v -test.run Discovery/discoveryv4/v4001  -simulatorHost "$HIVE_SIMULATOR" 


# Run hive (must be run from root directory of hive repo)
# hive --client=go-ethereum:master --test=NONE --sim=ethereum/rpc/eth --docker-noshell -loglevel 6
#
# Logs can be found in workspace/logs/simulations/ethereum/rpc\:eth\[go-ethereum\:develop\]/simulator.log
