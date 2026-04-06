# hive - Ethereum end-to-end test harness

Hive is a system for running integration tests against Ethereum clients.

Ethereum Foundation maintains two public Hive instances to check for consensus, p2p and
blockchain compatibility:

- eth1 consensus, EngineAPI, RPC tests, graphql and p2p tests are on <https://hive.ethpandaops.io>

**To read more about hive, please check [the documentation][doc].**

# Gnosis differences and features

Compared with the original hive, the repository contains gnosis specific tests and configurations.

Supported clients:

- Nethermind (`nethermind-gnosis`)
- Erigon (`erigon-gnosis`)
- Geth (`go-ethereum-gnosis`)
- Reth (`reth-gnosis`)

Supported simulators:

- gnosis/engine

The above simulator contains rewritten `ethereum/engine` tests with gnosis specific libraries and configurations.
Following suites are supported:

- engine-withdrawals
- engine-auth (nethermind only)
- engine-cancun
- engine-exchange-capabilities
- engine-api

The following simulators are used to run EEST tests:

- gnosis/eest/consume-engine
- gnosis/eest/consume-rlp

The Gnosis-specific EEST implementation is currently a work in progress.

## Scripts Module

This module contains various scripts to assist in the development of the project.

### run_tests.sh

This script allows us to run tests and store all logs, JRPC requests, and responses in a single location (the experiments directory). The script can:

1. Run tests against any combination of clients and simulators.
2. Store all logs, JRPC requests, and responses in one place (the experiments directory).
3. Run tests with a proxy to capture all JRPC requests and responses.
4. Copy the genesis file and other necessary files from the running client to a specified directory.

Usage:

```bash
./scripts/run_test.sh --test "/Blob Transaction Ordering, Single Account, Dual Blob" --exp "01" --client "nethermind-gnosis" --simulator "ethereum/gnosis-engine" --proxy "192.168.3.49:8089" > ./scripts/test.log

```

Options:

- t|--test - test regular expression to run, e.g. `/Blob Transaction Ordering, Single Account, Dual Blob`
- `-e|--exp - experiment number, directory inside scripts/experiments, e.g.`01
- `-c|--client - client name, e.g.`nethermind-gnosis
- `-s|--simulator - simulator name, e.g.`ethereum/engine
- `-p|--proxy - IP address of the local machine and mitmproxy port, e.g.`192.168.3.49:8089`

### Using mitmproxy

To use mitmproxy, we need to run it on the local machine and set the proxy URL (`HTTP_PROXY` environment variable). For example, `./scripts/run_test.sh -p 192.168.3.49:8089`.

The goal of the proxy is to capture all requests and responses between the client and the simulator. Additionally, it is possible to use scripts to gather data, pause, or modify requests and responses.

### Existing mitmproxy scripts

### stop_test.py

This script pauses the test execution after a specific condition is met. All configuration details are described in the scripts/proxy/config.ini file. Options:

- target_url - Docker server URL (`server.go`) that allows copying files between the container and the host machine.
- copy_files - If set to `True`, the script will copy files from the container to the host machine (needs also to specify `test_end`). •
- test_end - The endpoint of requests after which mitmproxy will pause the script, e.g., `/testsuite/1/test/3`.
- end_wait_time - The time in seconds the script will wait after resuming the test, e.g., `10`.

[copy_files] - This INI section contains files that should be copied from the container. The section consists of the following fields:

- container_file_path_1 = /genesis.json
- host_file_path_1 = scripts/experiments

To add additional files, use the same pattern, e.g.,

- container_file_path_2 = /mydir/otherfile.json
- host_file_path_2 = scripts/experiments

### server.go

The app is a simple HTTP server that allows to copy files and directories between the container and the host machine. To run the server, use the following command:

```bash
go run scripts/server.go -port 9090
```

Usage:

```bash
curl -X POST -H "Content-Type: application/json" -d '{
  "container_id": "d136581a900e",
  "container_file_path": "/genesis.json",
  "host_file_path": "scripts/experiments/01/genesis.json"
}' "http://localhost:9090/copyfile"

```

Will copy `/genesis.json` from the container with the id `d136581a900e` to the host machine to the `scripts/experiments/01/genesis.json`.

```bash
curl -X POST "http://localhost:8080/stop"

```

Will stop the server.

### clear_debug.sh

Clear `scripts/experiments/01` directory and move all files to `scripts/experiments/01-prev`.

### report_loader.sh

Load the archive of requests and responses from the `scripts/experiments/01` to the new mitmproxy instance. Load hive report.

## Sync with upstream

To support all ethereum features and new tests, we need to sync with upstream periodically.
The process is the following:

1. Merge `reflow` (`develop`) branch into `sync-upstream`
2. Go to `sync-upstream` branch and click on `Sync fork`
3. Create a pull request against `reflow` (`develop`) branch
4. Resolve merge conflicts, check changes of shared code (e.g. `ethereum/engine` tests)
5. If possible, update `gnosis-engine` simulator with the changes, if not - analyze and mention in the PR description to do later
6. Run the tests against `sync-upstream` branch using `Daily run Gnosis` workflow
7. If tests pass, merge the PR

## Run tests on CI

There are 3 Github actions [available](https://github.com/gnosischain/hive/actions) to run tests on CI:

- `Daily run Gnosis` (`dashboard_matrix.yml`) - runs all tests every day at 00:00 UTC
- `Debug Gnosis Tests` (`dashboard_debug.yml`) - specify client, simulator, suite and branch to run
- `Debug Gnosis Tests (detailed)` (`dashboard_debug_detailed.yml`) - the same as above but the user can specify an exact client version (docker image and tag) and test to run

After test run, the results are published on the [Gnosis dashboard](https://hive-gno.nethermind.io/).

### Trophies

If you find a bug in your client implementation due to this project, please be so kind as
to add it here to the trophy list. It could help prove that `hive` is indeed a useful tool
for validating Ethereum client implementations.

- go-ethereum:
  - Genesis chain config couldn't handle present but empty settings: [#2790](https://github.com/ethereum/go-ethereum/pull/2790)
  - Data race between remote block import and local block mining: [#2793](https://github.com/ethereum/go-ethereum/pull/2793)
  - Downloader didn't penalize incompatible forks harshly enough: [#2801](https://github.com/ethereum/go-ethereum/pull/2801)
- Nethermind:
  - Bug in p2p with bonding nodes algorithm found by Hive: [#1894](https://github.com/NethermindEth/nethermind/pull/1894)
  - Difference in return value for 'r' parameter in getTransactionByHash: [#2372](https://github.com/NethermindEth/nethermind/issues/2372)
  - CREATE/CREATE2 behavior when account already has max nonce [#3698](https://github.com/NethermindEth/nethermind/pull/3698)
  - Blake2 performance issue with non-vectorized code [#3837](https://github.com/NethermindEth/nethermind/pull/3837)
- Besu:
  - Missing v result for blob and pending tx [#8196](https://github.com/hyperledger/besu/pull/8196)
  - EIP-7702 - skip CodeDelegation processing for invalid recid [#8212](https://github.com/hyperledger/besu/pull/8212)
  - LogTopic - empty list is wildcard topic [#8420](https://github.com/hyperledger/besu/pull/8420)
  - RLP Block Importer - move worldstate head only if import successful [#8447](https://github.com/hyperledger/besu/pull/8447)
  - Bug in Bonsai Archive mode when storage to delete could be null: [#8434](https://github.com/hyperledger/besu/pull/8434)
  - Bug in estimating gas - if no gas params set, tx was being estimated as a FRONTIER tx but should be 1559 [#8472](https://github.com/hyperledger/besu/pull/8472)

## Contributions

This project takes a different approach to code contributions than your usual FOSS project
with well ingrained maintainers and relatively few external contributors. It is an
experiment. Whether it will work out or not is for the future to decide.

We follow the [Collective Code Construction Contract (C4)][c4], code contribution model,
as expanded and explained in [The ZeroMQ Process][zmq-process]. The core idea being that
any patch that successfully solves an issue (bug/feature) and doesn't break any existing
code/contracts must be optimistically merged by maintainers. Followup patches may be used
for additional polishes – and patches may even be outright reverted if they turn out to
have a negative impact – but no change must be rejected based on personal values.

### License

The hive project is licensed under the [GNU General Public License v3.0][gpl]. You can
find it in the COPYING file.

[doc]: ./docs/overview.md
[c4]: http://rfc.zeromq.org/spec:22/C4/
[zmq-process]: https://hintjens.gitbooks.io/social-architecture/content/chapter4.html
[gpl]: http://www.gnu.org/licenses/gpl-3.0.en.html
