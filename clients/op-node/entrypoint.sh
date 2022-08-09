#!/bin/bash

# Hive requires us to prefix env vars with "HIVE_"
# Iterate the env, find all HIVE_UNPACK_ vars, and remove the HIVE_UNPACK_ prefix.
while IFS='=' read -r -d '' n v; do
    if [[ "$n" == HIVE_UNPACK_* ]]; then
        name=${n#"HIVE_UNPACK_"}  # remove the HIVE_UNPACK_ prefix
        echo "$name=$v"
        declare -gx "$name=$v"
    fi
done < <(env -0)

op-node


