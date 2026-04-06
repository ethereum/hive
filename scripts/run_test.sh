#!/bin/bash
# set -x
# Default values
TEST_NAME="/Blob Transaction Ordering, Single Account, Dual Blob"
EXPERIMENT="debug"
CLIENT="nethermind-gnosis"
SIMULATOR="gnosis/engine"
SIMULATOR_LOG_LEVEL=3
CLIENT_LOG_LEVEL=3
PROXY=""
MITMPROXY_ADDITIONAL_ARGS='-s "/home/mitmproxy/.mitmproxy/stop_test.py"'
MITMPROXY_CONFIG_NAME="config"
PROXY_CONTAINER_NAME="test_run_proxy"
GO_SERVER_PORT=9096
DEBUG=0

while [[ $# -gt 0 ]]; do
  case $1 in
    -t|--test)
      TEST_NAME="$2"
      shift # past argument
      shift # past value
      ;;
    -e|--exp)
      EXPERIMENT="$2"
      shift
      shift
      ;;
    -c|--client)
      CLIENT="$2"
      shift
      shift
      ;;
    -s|--simulator)
      SIMULATOR="$2"
      shift
      shift
      ;;
    -p|--proxy)
      PROXY="$2"
      shift
      shift
      ;;
    -pc|--proxy-config)
      MITMPROXY_CONFIG_NAME="$2"
      shift
      shift
      ;;
    --debug)
      DEBUG=1
      shift
      shift
      ;;
    *)    # unknown option
      shift # past argument
      ;;
  esac
done

echo "Started test: '$TEST_NAME' and experiment: '$EXPERIMENT' with client: '$CLIENT' and simulator: '$SIMULATOR'"

echo "Starting docker server on port $GO_SERVER_PORT"
go run scripts/server.go -port $GO_SERVER_PORT &

if [ "$PROXY" == "" ]; then
  echo "Proxy not set"
else
  docker stop $PROXY_CONTAINER_NAME || echo "Container $PROXY_CONTAINER_NAME does not started"
  docker rm "$PROXY_CONTAINER_NAME" || echo "Container $PROXY_CONTAINER_NAME does not exist"
  # Get mitmproxy container ID
  CONTAINER_ID=$(docker run --name $PROXY_CONTAINER_NAME -d -e MITMPROXY_EXPERIMENT_ID=$EXPERIMENT -e MITMPROXY_CONFIG_NAME=$MITMPROXY_CONFIG_NAME -p 7080:8080 -p 8089:8089 -p 8082:8082 -v $PWD/scripts/proxy:/home/mitmproxy/.mitmproxy mitmproxy/mitmproxy mitmweb --listen-host 0.0.0.0 --web-host 0.0.0.0 --listen-port 8089 --web-port 8082 --set ssl_insecure=true $MITMPROXY_ADDITIONAL_ARGS --set hardump="/home/mitmproxy/.mitmproxy/$EXPERIMENT.har")
  echo "Using proxy: $PROXY, container id: $CONTAINER_ID, config: scripts/proxy/$MITMPROXY_CONFIG_NAME.ini, container name: $PROXY_CONTAINER_NAME"
  # Needs some time to start the proxy
  sleep 5
fi

DOCKER_FILE="Dockerfile"
if [ "$DEBUG" == 1 ]; then
  DOCKER_FILE="Dockerfile.debug"
fi

HIVE_TTD_ENABLED=false HTTP_PROXY="$PROXY" ./hive --sim "$SIMULATOR" --sim.limit "$TEST_NAME" --client "$CLIENT" --loglevel="$CLIENT_LOG_LEVEL" --sim.loglevel="$SIMULATOR_LOG_LEVEL" --docker.output --results-root="scripts/experiments/$EXPERIMENT/runs" --dev.addr="127.0.0.1:3000" --docker.override-dockerfile="$DOCKER_FILE" --docker.pull

if [ "$PROXY" == "" ]; then
  echo "No need to stop proxy"
else
  docker stop "$CONTAINER_ID"
  # Wait some time to get all the logs
  sleep 5
  docker logs "$CONTAINER_ID"
  mv "scripts/proxy/$EXPERIMENT.har" "scripts/experiments/$EXPERIMENT"
fi

echo "Stopping docker server"
curl -X POST "http://localhost:$GO_SERVER_PORT/stop"

echo "Finished test: '$TEST_NAME' and experiment: '$EXPERIMENT'"