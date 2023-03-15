# Hive Pyspec

This is a `simulator` for running the python [execution-spec-tests](https://github.com/ethereum/execution-spec-tests) within [`hive`](https://github.com/ethereum/hive), based on the `consensus` simulator. It differs mostly by using the `Engine API` to feed blocks into clients. 

The simulator is only valid for post `Merge` forks (>= `Merge`).

It can be run from the `hive` root directory simply:
```sh
./hive --client go-ethereum --sim ethereum/pyspec
```

The `pyspec simulator` uses the latest test fixtures from the
most recent execution-spec-tests [release](https://github.com/ethereum/execution-spec-tests/releases).


### Running Specific Test Fixtures

By default all test fixtures will run. To run a specific test or set of test fixtures the regex `--sim-limit /<pattern>` option can be used.

This can utilised in `pyspec` for running tests exclusive to  a specific fork. The pattern must match at least one hive simulation test name within the `pyspec` suite.

Test names are of the type `<path-to-json>_<fixture-name>` omiting `fixtures` from the path. For example, the fixture test `000_push0_key_sstore_shanghai` within `push0.json` will have a hive test name of: `eips_eip3855_000_push0_key_sstore_shanghai`.

1) To run only this test, an example pattern could be:
   - `./hive --sim ethereum/pyspec --sim.limit /0_push0_key_sstore`

2) To run every withdrawals test in the suite:
   - `./hive --sim ethereum/pyspec --sim.limit /withdrawals`

3) To run only tests exclusive to the `Shanghai` fork:
   - `./hive --sim ethereum/pyspec --sim.limit /shanghai`

For a better understand of how to utilise the regex pattern please browse the folder structure of the latest fixture [release](https://github.com/ethereum/execution-spec-tests/releases).


### Excluding Test Fixtures

To exclude a file or set of files, partial paths can be added to `excludePaths := []string{"example/"}` within \
the first code section of `runner.go/LoadFixtureTests()`. By default we exclude all files within the path `fixtures/example/example/*`. 

