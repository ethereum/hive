#!/bin/bash

failed=""
sims=$(find simulators/ -name go.mod -printf '%h\n')
for d in $sims; do
    ( cd $d; go build . )
    if [ $? -ne 0 ]; then
        failed="$d"
    fi
done

# Exit with non-zero status code if any build failed.
[ -n $failed ] && exit 1
