#!/bin/bash

# Immediately abort the script on any error encountered
set -e

# Send CTRL+C to start the shutdown process
kill -SIGINT $(pgrep beacon-chain)

# Wait for graceful shutdown
TIMEOUT_SECONDS=20
while pgrep beacon-chain; do
    if (( TIMEOUT_SECONDS == 0 )); then
        break
    fi
    sleep 1
    (( TIMEOUT_SECONDS-- ))
done

# Forcefully kill if it did not shutdown
if pgrep beacon-chain; then
    kill -9 $(pgrep beacon-chain)
fi