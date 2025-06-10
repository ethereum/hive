#!/bin/sh

wd="$(pwd)"
cd ../..
go build ./cmd/hivechain

./hivechain generate \
    --outdir "$wd/chain" \
    --fork-interval 6 \
    --tx-interval 1 \
    --length 500 \
    --lastfork cancun \
    --outputs accounts,genesis,chain,headstate,txinfo,headblock,headfcu,newpayload,forkenv
