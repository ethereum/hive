#!/bin/sh

wd="$(pwd)"
cd ../../..
go build ./cmd/hivechain
./hivechain generate \
    -pos \
    -length 3000 \
    -tx-interval 5 \
    -fork-interval 0 \
    -finalized-distance 50 \
    -outdir "$wd/chain" \
    -lastfork cancun \
    -outputs forkenv,genesis,chain,headblock,headfcu,headnewpayload
