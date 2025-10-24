#!/usr/bin/env bash

set -e

for contract in contracts/*.eas; do
    output=bytecode/$(basename ${contract%.*}).bin
    echo $contract "->" $output
    go tool geas -a -bin -o $output $contract
done
