To generate these .json and rlp files, run 

```sh
go build ./cmd/hivechain

./hivechain generate -fork-interval 0 -tx-interval 1 -length 45 -outdir ./simulators/berachain/berachain-rpc-compat/tests/ -outputs genesis,chain,headfcu,accounts,forkenv,headblock,txinfo -berachain
```