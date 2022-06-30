#!/bin/bash

version=optimism
if [ -n "$1" ]; then
    version="$1"
fi

sims=$(find simulators -name go.mod)
for d in $sims; do
    d="$(dirname $d)"
    echo "upgrading hivesim in $d"
    (
        set -e
        cd $d
        go get -d "github.com/ethereum-optimism/hive@$version"
        go mod tidy
    )
done
