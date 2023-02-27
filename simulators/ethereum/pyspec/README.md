# Hive Pyspec

This is a `simulator` for running the python [execution-spec-tests](https://github.com/ethereum/execution-spec-tests) within [`hive`](https://github.com/ethereum/hive), based on the `consensus` simulator. It differs mostly by using the `Engine API` to feed blocks into clients. 

It can be run from the `hive` root directory simply, e.g for `geth`:
```sh
./hive --client go-ethereum --sim ethereum/pyspec
```

The `pyspec simulator` uses the latest test fixtures from the
most recent execution-spec-tests [release](https://github.com/ethereum/execution-spec-tests/releases).
