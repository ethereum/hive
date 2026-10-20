[Overview] | [Hive Commands] | [Simulators] | [Clients]

## Installing Hive

We have not tested hive on any OS other than Linux. It is usually best to use Ubuntu or
Debian.

First add `build-essential` by `sudo apt install build-essential`. Also install Go
version 1.17 or later and add it to your `$PATH` as described in the [Go installation
documentation]. You can check the installed Go version by running `go version`.

To get hive, you first need to clone the repository to the location of your choice.
Then you can build the hive executable.

    git clone https://github.com/ethereum/hive
    cd ./hive
    go build .

To run simulations, you need a working Docker setup. Hive needs to be run on the same
machine as dockerd. Using docker remotely is not supported at this time. [Install docker]
and add your user to the `docker` group to allow using docker without `sudo`.

    sudo usermod -a -G docker <user_name>

## Running Hive

All hive commands should be run from within the root of the repository. To run a
simulation, use the following command:

    ./hive --sim <simulation> --client <client(s) you want to test against>

For example, if you want to run the `discv4` test against geth, here is
how the command would look:

    ./hive --sim devp2p --sim.limit discv4 --client go-ethereum,nethermind

The client list may contain any number of clients. You can select a specific client
version by appending it to the client name with `_`, for example:

    ./hive --sim devp2p --client go-ethereum_v1.9.22,go-ethereum_v1.9.23

### Client Build Parameters

The client list for a run can also be given in a YAML file. This also allows further
customization of the build arguments of the client. Specify the `--config` option (or its
alias `--client-file`) to use a configuration file.

    ./hive --sim my-simulation --config clients.yaml

When `--client` is also given, it selects entries from the file by client name or by name
and nametag, for example `go-ethereum_main`. Without it, all client entries in the file
are used.

Here is an example clients.yaml file:

    - client: go-ethereum
      dockerfile: git
    - client: nethermind
      build_args:
        baseimage: nethermindeth/hive
        tag: latest

For each client in the list, the following options can be given:

- `client`: Name of the client. This must refer to a known client in the clients/ directory.
- `dockerfile`: The Dockerfile extension to use. For example, specifying `git` here will
   build the client using `Dockerfile.git` instead of the default `Dockerfile`.
- `nametag`: this can be used to assign a more descriptive name to the client. If unset,
   a unique nametag will be chosen based on the version tag and/or build arguments.
- `build_args`: Build arguments passed to the Dockerfile, see below.

Supported build arguments depend on the client and the docker image being used. Common build
arguments are:

- `tag`: The git commit/tag/branch or docker tag name to use.
- `baseimage`: For clients pulled from DockerHub, this can be used to override the organization
   and image name. Example `ethereum/client-go`.
- `github`: For client Dockerfiles building from git, this setting can be used to change
   the source code repository (fork) on GitHub. Example: `ethereum/go-ethereum`.

### Simulator Build Parameters

The configuration file can also carry simulator build configurations. Simulator entries
use the `simulator` key in place of `client` and can be mixed with client entries in the
same file, so a single `--config` file describes the whole run:

    ./hive --config devnet.yaml

Each simulator entry supports:

- `simulator`: A known simulator directory under simulators/. Each simulator may appear once.
- `dockerfile`: An optional extension, such as `git` for `Dockerfile.git`. The selected
  file must exist in the simulator directory. If omitted, uses `Dockerfile`.
- `build_args`: Arguments passed to the simulator's Dockerfile. Supported arguments and
  their defaults are the `ARG`s declared in that Dockerfile.

For example, this devnet.yaml runs two EELS simulators against go-ethereum, one from the
image published by execution-specs and one built from source. The next section describes
both:

    - client: go-ethereum
      build_args:
        baseimage: docker.io/ethereum/client-go
        tag: latest
    - simulator: ethereum/eels/consume-engine
      build_args:
        tag: glamsterdam-devnet-v8.1.4
    - simulator: ethereum/eels/consume-rlp
      dockerfile: git
      build_args:
        branch: devnets/glamsterdam/8
        fixtures: tests-glamsterdam-devnet@v8.1.4

Configured simulators run in file order unless `--sim` is supplied to filter them using
its usual regular expression matching. Repeated `--sim.buildarg NAME=VALUE` options apply
to every selected simulator and override matching arguments from the file. The simulator's
`hive_context.txt`, if present, still determines its build context. In `--dev` mode,
simulators are not built or run. A file without simulator entries leaves simulator
selection to `--sim`, and a file without client entries leaves client selection to
`--client`.

### EELS Simulator Build Parameters

The `ethereum/eels/*` simulators run the `consume` and `execute` commands of
[execution-specs] and have two Dockerfiles:

- `Dockerfile`, the default, runs from an image published by execution-specs under
  `ghcr.io/ethereum/execution-specs/hive/<simulator>`. The image contains the simulator
  code and the tests of one release, so hive pulls it and adds metadata layers only.
- `Dockerfile.git`, selected with `dockerfile: git`, clones execution-specs, installs
  the simulator with `uv sync` and, for the `consume-*` simulators, downloads a fixture
  release. Use it for an execution-specs branch or a fixture set that is not published
  as an image.

`Dockerfile` accepts:

- `tag`: The image tag. It selects the tests. A release tag is immutable; a channel
  such as `latest` pairs the highest release with the head of its branch. Defaults to
  `latest`. `<tag>@sha256:<digest>` pins an exact image.
- `image`: The image name, to use another registry namespace or a locally built image.
  Defaults to the published image of the simulator.

`Dockerfile.git` accepts:

- `branch`: The execution-specs Git ref to build from. Empty, the default, uses the
  repository's default branch.
- `fixtures`: The fixture input passed to `consume --input`: a release name such as
  `tests@v20.0.2`, or a URL. Defaults to `tests@latest`, the latest mainnet release.
  `consume-*` only.

Both files accept `disable_strict_exception_matching` on `consume-engine` and
`consume-enginex`, which defaults to `nimbus-el` and is passed to consume's corresponding
option, and `fork` on `execute-blobs`. `Dockerfile` defaults `fork` to the fork the
image's release was filled for; `Dockerfile.git` has its own default.

Mainnet release tags omit `tests@`; devnet release tags omit `tests-` and replace
`@` with `-`. Channel tags follow the releases of a line. For consume simulators,
the release name is the `fixtures` input for `Dockerfile.git`, and the branch the
release was cut from is its `branch`:

| `tag` | Release | `branch` |
| --- | --- | --- |
| `v20.0.2` | `tests@v20.0.2`, mainnet release 20.0.2 | empty, the default branch |
| `glamsterdam-devnet-v8.1.4` | `tests-glamsterdam-devnet@v8.1.4`, release 8.1.4 of Glamsterdam devnet 8 | `devnets/glamsterdam/8` |
| `glamsterdam-devnet-8` | the highest `tests-glamsterdam-devnet@v8.*` release | `devnets/glamsterdam/8` |
| `latest` | the highest `tests@v*` release, as `tests@latest`; the default | empty, the default branch |
| `nightly` | the most recent nightly fill of the default branch; no release | the default branch |

Release tags never change. Channels receive framework updates on branch pushes, so
they also pick up fixes such as client exception mappings. Nightly images use the
nightly fill's commit.
`execute-blobs` source builds run the tests from `branch`, while release images
contain the release's test sources. Existing `branch` and `fixtures` arguments are
deprecated on the default Dockerfiles: without `tag` they still select a source build
of the framework and fixtures, with a deprecation notice, until they are removed.
Use `tag`, or `dockerfile: git`.

Channels and `nightly` move; `tag=<tag>@sha256:<digest>` runs an exact image. The full
tag scheme is described in the [EELS simulators README] and in the execution-specs
documentation under `docs/running_tests/hive/images/`.

### Docker Options

`--docker.pull`: Setting this option makes hive re-pull the base images of all built
docker containers.

`--docker.output`: This enables printing of all docker container output to stderr.

`--docker.nocache <expression>`: Regular expression selecting docker images to forcibly
rebuild. You can use this option during simulator development to ensure a new image is
built even when there are no changes to the simulator code.

### Simulation Options

`--sim.limit <pattern>`: Specifies a regular expression to selectively enable suites and
test cases. This is interpreted by simulators. It sets the `HIVE_TEST_PATTERN` environment
variable.

The test pattern expression is usually interpreted as an unanchored match, i.e. an empty
pattern matches any suite/test name and the expression can match anywhere in name. To
improve command-line ergonomics, the test pattern is split at the first occurrence of `/`.
The part before `/` matches the suite name and everything after it matches test names.

For example, the following command runs the `devp2p` simulator, limiting the run to the
`eth` suite and selecting only tests containing the word `Large`.

    ./hive --sim devp2p --sim.limit eth/Large

This command runs the `consensus` simulator and runs only tests from the `stBugs`
directory (note the first `/`, matching any suite name):

    ./hive --sim ethereum/consensus --sim.limit /stBugs/

`--sim.timelimit <timeout>`: Simulation timeout. Hive aborts the simulator if it exceeds
this time. There is no default timeout.

`--client.checktimelimit <timeout>`: The timeout of waiting for clients to open up TCP
port 8545. If a very long chain is imported, this timeout may need to be quite long. A
lower value means that hive won't wait as long in case the node crashes and never opens
the RPC port. Defaults to 3 minutes.

`--sim.loglevel <level>`: Selects log level of client instances. Supports values 0-5,
defaults to 3. Note that this value may be overridden by simulators for specific clients.
This sets the default value of `HIVE_LOGLEVEL` in client containers.

`--sim.parallelism <number>`: Sets max number of parallel clients/containers. This is
interpreted by simulators. It sets the `HIVE_PARALLELISM` environment variable. Defaults
to 1.

`--sim.randomseed <number>`: Sets a fixed number as the randomness seed to be used by all
simulators. It sets the `HIVE_RANDOM_SEED` environment variable. Defaults to zero, which
translates being unset and the simulators decide the source of randomness.

## Viewing simulation results (hiveview)

The results of hive simulation runs are stored in JSON files containing test results, and
hive also creates several log files containing the output of the simulator and clients. To
view test results and logs in a web browser, you can use the `hiveview` tool. Build it
with:

    go build ./cmd/hiveview

Run it like this to start the HTTP server:

    ./hiveview --serve --logdir ./workspace/logs

This command runs a web interface on <http://127.0.0.1:8080>. The interface shows
information about all simulation runs for which information was collected.

## Generating Ethereum 1.x test chains (hivechain)

The `hivechain` tool allows you to create RLP-encoded blockchains for inclusion into
simulations. Build it with:

    go build ./cmd/hivechain

To generate a chain of a desired length, run the following command:

    ./hivechain generate -length 200

hivechain creates its own genesis block, so no genesis file needs to be supplied. By
default, it writes `genesis.json`, `chain.rlp` and `txinfo.json` to the current directory
(use `-outdir` to change this) and adds transactions every 10 blocks (see `-tx-interval`
and `-tx-count`). The pre-funded accounts and their private keys can be written to
`accounts.json` by including `accounts` in `-outputs`. Refer to the [hivechain README]
for all generator options and output formats.

[Go installation documentation]: https://golang.org/doc/install
[Install docker]: https://docs.docker.com/engine/install/debian/#install-using-the-repository
[execution-specs]: https://github.com/ethereum/execution-specs
[EELS simulators README]: ../simulators/ethereum/eels/README.md
[Overview]: ./overview.md
[Hive Commands]: ./commandline.md
[Simulators]: ./simulators.md
[Clients]: ./clients.md
[hivechain README]: ../cmd/hivechain/README.md
