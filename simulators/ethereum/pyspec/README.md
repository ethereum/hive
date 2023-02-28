# Hive Pyspec

This is a `simulator` for running the python [execution-spec-tests](https://github.com/ethereum/execution-spec-tests) within [`hive`](https://github.com/ethereum/hive), based on the `consensus` simulator. It differs mostly by using the `Engine API` to feed blocks into clients. 

The simulator is only valid for post `Merge` forks.

It can be run from the `hive` root directory simply, e.g for `geth`:
```sh
./hive --client go-ethereum --sim ethereum/pyspec
```

The `pyspec simulator` uses the latest test fixtures from the
most recent execution-spec-tests [release](https://github.com/ethereum/execution-spec-tests/releases).


### Running Specific Test Fixtures

To run a specific or set of test fixtures `testCategory := ""`, within the third code section of `main.go/fixtureRunner()`, can be updated:
- To only run the `large_withdrawals.json` test fixture: `testCategory := "withdrawals/withdrawals/large_withdrawals.json"`.
- To run all the withdrawals test fixtures: `testCategory := "withdrawals"`.

### Excluding Test Fixtures

To exclude a file or set of files, partial paths can be added to `excludePaths := []string{"example/"}` within \
the first code section of `runner.go/LoadFixtureTests()`. By default we exclude all files within the path `fixtures/example/example/*`. 

