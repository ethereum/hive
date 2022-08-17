#!/bin/bash

# Immediately abort the script on any error encountered
set -e

# Send CTRL+C to start the shutdown process
kill -SIGINT $(pgrep nimbus_beacon_node)

# Wait for graceful shutdown
TIMEOUT_SECONDS=20
while pgrep nimbus_beacon_node; do
    if (( TIMEOUT_SECONDS == 0 )); then
        break
    fi
    sleep 1
    (( TIMEOUT_SECONDS-- ))
done

# Forcefully kill if it did not shutdown
if pgrep nimbus_beacon_node; then
    kill -9 $(pgrep nimbus_beacon_node)
fi