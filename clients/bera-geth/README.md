## Changes

1. Copy the `clients/go-ethereum` directory to create `clients/bera-geth`:

```sh
cd clients/
cp -r go-ethereum bera-geth
```

2. Update the base image and version in `Dockerfile`:

```diff
- ARG baseimage=ethereum/client-go
- ARG tag=latest

+ # Pull bera-geth image
+ ARG baseimage=ghcr.io/berachain/bera-geth
+ ARG tag=v1.011602.1
```

3. Replace the `geth` binary with `bera-geth` in `Dockerfile`:

```diff
- COPY --from=builder /usr/local/bin/geth /usr/local/bin/geth

+ # Use bera-geth instead of geth
+ COPY --from=builder /usr/local/bin/bera-geth /usr/local/bin/geth
```

## Running simulation tests

Simulations from [`devp2p`](../../simulators/devp2p/) and [`ethereum`](../../simulators/ethereum/) were executed to compare the behavior of `bera-geth` with `go-ethereum`. Results indicate that `bera-geth` operates equivalently in these scenarios.


| Test              | Output of `bera-geth` | Output of `go-ethereum` | Command                                                       | Comments                                                                                                                                                                                                                                                                                                                                                                    |
|-------------------|-----------------------|--------------------------|---------------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `devp2p`          | 82/100                | 82/100                   | `./hive --sim devp2p --client go-ethereum,bera-geth`          | - `discv4` suite: 32 succeeded, 0 failed <br> - `discv5` suite: 6 succeeded, 10 failed <br> - `eth` suite: 36 succeeded, 4 failed <br> - `snap` suite: 8 succeeded, 4 failed                                                                                                                                                                                                |
| `ethereum/engine` | 641/801               | 641/801                  | `./hive --sim ethereum/engine --client go-ethereum,bera-geth` | - `engine-api` suite: 257 succeeded, 0 failed <br> - `engine-auth` suite: 15 succeeded, 0 failed <br> - `engine-exchange-capabilities` suite: 9 succeeded, 0 failed <br> - `engine-withdrawals` suite: 29 succeeded, 40 failed <br> - `engine-cancun` suite: 331 succeeded, 120 failed                                                                                       |
