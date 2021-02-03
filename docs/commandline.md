[Overview] | [Hive Commands] | [Simulators] | [Clients]

## Running Hive

The hive project is implemented in Go. You need to install Go version 1.13 or later to use
hive. To run simulations, you also need a working Docker setup, and hive needs to be run on the
same machine as dockerd. Using docker remotely is not supported at this time. We have also
not tested hive extensively on any OS but Linux, so you must run Linux to use hive.

To get hive, you first need to clone to the repository to any location, then build the hive
executable.

    git clone https://github.com/ethereum/hive
    cd ./hive
    go build .

All hive commands should be run from within the root of the repository. To run a
simulation, use the following command:

    ./hive --sim <simulation> --client <client(s) you want to test against>

For example, if you want to run the `discv4` test against geth and openethereum, here is
how the command would look:

    ./hive --sim devp2p/discv4 --client go-ethereum,openethereum

The client list may contain any number of clients. You can select a specific client
version by appending it to the client name with an `_`, for example:

    ./hive --sim devp2p/discv4 --client go-ethereum_v1.9.22,go-ethereum_v1.9.23

Simulation runs can be customized in many ways. Here's an overview of the available
command-line options.

`--client.checktimelimit <timeout>`: The timeout of waiting for clients to open up TCP
port 8545. If a very long chain is imported, this timeout may need to be quite large. A
lower value means that hive won't wait as long in case the node crashes and never opens
the RPC port. Defaults to 3 minutes.

`--docker.pull`: Setting this option makes hive re-pull the base images of all built
docker containers.

`--docker.output`: This enables printing of all docker container output to stderr.

`--docker.nocache <expression>`: Regular expression selecting docker images to forcibly
rebuild. You can use this option during simulator development to ensure a new image is
built even when there are no changes to the simulator code.

`--sim.timelimit <timeout>`: Simulation timeout. Hive aborts the simulator if it exceeds
this time. There is no default timeout.

`--sim.loglevel <level>`: Selects log level of client instances. Supports values 0-5,
defaults to 3. Note that this value may be overridden by simulators for specific clients.
This sets the default value of `HIVE_LOGLEVEL` in client containers.

`--sim.parallelism <number>`: Sets max number of parallel clients/containers. This is
interpreted by simulators. It sets the `HIVE_PARALLELISM` environment variable. Defaults
to 1.

`--sim.testlimit <number>`: Max number of tests to execute per client. This is interpreted
by simulators. It sets the `HIVE_SIMLIMIT` environment variable.

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

    hivechain generate -genesis ./genesis.json -length 200

hivechain generates empty blocks by default. The chain will contain non-empty blocks if
the following accounts have balance in genesis state. You can find the corresponding
private keys in the hivechain source code.

- `0x71562b71999873DB5b286dF957af199Ec94617F7`
- `0x703c4b2bD70c169f5717101CaeE543299Fc946C7`
- `0x0D3ab14BBaD3D99F4203bd7a11aCB94882050E7e`

[Overview]: ./overview.md
[Hive Commands]: ./commandline.md
[Simulators]: ./simulators.md
[Clients]: ./clients.md
