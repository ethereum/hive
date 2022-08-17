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

Please see the `execution-apis` testing [documentation][tests].

[tests]: https://github.com/ethereum/execution-apis/tree/main/tests
