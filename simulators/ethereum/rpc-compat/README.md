# `rpc-compat` Simulator

The `rpc-compat` simulator runs a suite of [conformance tests][tests] against
all available clients. Its primary objective is to ensure clients return the
same values over JSON-RPC so that they may be treated as a black box by
downstream tooling such as wallets and dapps.

## Getting Started

To run the simulator locally, first build `hive`.

```
go build
```

Then start the simulator.

```
./hive --sim ethereum/rpc-compat --client go-ethereum,besu,nethermind
```

### Running local tests

To run a set of local tests instead of the standard conformance test suite,
place the `tests` directory into the `rpc-compat` simulator directory and
uncomment the lines in the `Dockerfile` associated with adding the directory to
the build process and then comment out the `git clone` command. Now, the above
`hive` command can be executed again.

## Test Generation

Fixtures using `engine_*` methods are sent to port 8551 with Hive's test JWT
secret. Other methods use the public RPC endpoint on port 8545. The ordinary
`>>` / `<<` fixture format is unchanged.

Response checking uses the existing schema, literal comparison, and fixture
validation-script mechanisms. There is no Engine-specific response rewriting.
Fixtures with client-specific values such as `payloadId` can use `speconly`
with a validation script that checks the ID format and all other expected
fields. Schema validation alone does not establish a successful payload build.

Engine fixtures must use a chain with the corresponding fork activated. This
routing support alone does not activate forks or supply payload attributes.

Please see the `execution-apis` testing [documentation][tests].

[tests]: https://github.com/ethereum/execution-apis/tree/main/tests
