#!/bin/bash

# Hive requires us to prefix env vars with "HIVE_"
# Iterate the env, find all HIVE_ROLLUP_NODE_ vars, and remove the HIVE_ prefix.
env -0 | while IFS='=' read -r -d '' n v; do
    if [[ "$n" == HIVE_ROLLUP_NODE_* ]]; then
        name=${n#"HIVE_"}  # remove the HIVE_ prefix
        printf "'%s'='%s'\n" "$name" "$v"
        echo "$name=$v"
        export "$name=$v"
    fi
done

exec op-node


