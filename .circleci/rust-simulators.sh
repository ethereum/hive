#!/bin/bash

# This causes the bash script to exit immediately if any commands errors out
set -e

failed=""
sims=$(find simulators -name Cargo.toml)
for d in $sims; do
    d="$(dirname "$d")"
    echo "Lint, build, test $d"
    ( cd "$d" || exit 1;
    cargo fmt --all -- --check;
    cargo clippy --all --all-targets --all-features --no-deps -- --deny warnings;
    cargo test --workspace -- --nocapture;
    )
done
