[Overview] | [Hive Commands] | [Simulators] | [Clients]

## Hive Clients

This page explains how client containers work in Hive.

Clients are docker images which can be instantiated by a simulation. A client definition
consists of a Dockerfile and associated resources. Client definitions live in
subdirectories of `clients/` in the hive repository.

See the [go-ethereum client definition][geth-docker] for an example of a client
Dockerfile.

When hive runs a simulation, it first builds all client docker images using their
Dockerfile, i.e. it basically runs `docker build .` in the client directory. Since most
client definitions wrap an existing Ethereum client, and building the client from source
may take a long time, it is usually best to base the hive client wrapper on a pre-built
docker image from Docker Hub.

The client Dockerfile should support an optional argument named `branch`, which specifies
the requested client version. This argument can be set by users by appending it to the
client name like:

    ./hive --sim my-simulation --client go-ethereum_v1.9.23,go_ethereum_v1.9.22

Other build arguments can also be set using a YAML file, see the [hive command
documentation][hive-client-yaml] for more information.

### Alternative Dockerfiles

There can be other Dockerfiles besides the main one. Typically, a client should also
provide a `Dockerfile.git` that builds the client from source code. Alternative
Dockerfiles can be selected through hive's `-client-file` YAML configuration.

### hive.yaml

Hive reads additional metadata from the `hive.yaml` file in the client directory (next to
the Dockerfile). Currently, the only purpose of this file is specifying the client's role
list:

    roles:
      - "eth1"
      - "eth1_light_client"

The role list is available to simulators and can be used to differentiate between clients
based on features. Declaring a client role also signals that the client supports certain
role-specific environment variables and files. If `hive.yaml` is missing or doesn't declare
roles, the `eth1` role is assumed.

### /version.txt

Client Dockerfiles are expected to generate a `/version.txt` file during build. Hive reads
this file after building the container and attaches version information to the output of
all test suites in which the client is launched.

### /hive-bin

Executables placed into the `/hive-bin` directory of the client container can be invoked
through the simulation API.

## Client Lifecycle

When the simulation requests a client instance, hive creates a docker container from the
client image. The simulator can customize the container by passing environment variables
with prefix `HIVE_`. It may also upload files into the container before it starts. Once
the container is created, hive simply runs the entry point defined in the `Dockerfile`.

For all client containers, hive waits for TCP port 8545 to open before considering the
client ready for use by the simulator. This port is configurable through the
`HIVE_CHECK_LIVE_PORT` variable, and the check can be disabled by setting it to `0`. If
the client container does not open this port within a certain timeout, hive assumes the
client has failed to start.

Environment variables and files interpreted by the entry point define a 'protocol' between
the simulator and client. While hive itself does not require support for any specific
variables or files, simulators usually expect client containers to be configurable in
certain ways. In order to run tests against multiple Ethereum clients, for example, the
simulator needs to be able to configure all clients for a specific blockchain and make
them join the peer-to-peer network used for testing.

## Eth1 Client Requirements

This section describes the requirements for the `eth1` client role.

Eth1 clients must provide JSON-RPC over HTTP on TCP port 8545. They may also support
JSON-RPC over WebSocket on port 8546, but this is not strictly required.

### Files

The simulator customizes client startup by placing these files into the eth1 client
container:

- `/genesis.json` contains Ethereum genesis state in the JSON format used by Geth. This
  file is mandatory.
- `/chain.rlp` contains RLP-encoded blocks to import before startup.
- `/blocks/` directory containing `.rlp` files.

On startup, the entry point script must first load the genesis block and state into the
client implementation from `/genesis.json`. To do this, the script needs to translate from
Geth genesis format into a format appropriate for the specific client implementation. The
translation is usually done using a jq script. See the [openethereum genesis
translator][oe-genesis-jq], for example.

After the genesis state, the client should import the blocks from `/chain.rlp` if it is
present, and finally import the individual blocks from `/blocks` in file name order. The
reason for requiring two different block sources is that specifying a single chain is more
optimal, but tests requiring forking chains cannot create a single chain. The client
should start even if the blocks are invalid, i.e. after the import, the client's 'best
block' should be the last valid, imported block.

### Scripts

Some tests require peer-to-peer node information of the client instance. All eth1 client
containers must contain a `/hive-bin/enode.sh` script. This script should output the enode
URL of the running instance.

### Environment

Clients must support the following environment variables. The client's entry point script
may map these to command line flags or use them to generate a config file, for example.

| Variable                   | Value         |                                                |
|----------------------------|---------------|------------------------------------------------|
| `HIVE_LOGLEVEL`            | 0 - 5         | configures log level of client                 |
| `HIVE_NODETYPE`            | archive, full | sets sync algorithm                            |
| `HIVE_BOOTNODE`            | enode URL     | makes client connect to another node           |
| `HIVE_GRAPHQL_ENABLED`     | 0 - 1         | if set, GraphQL is enabled on port 8545        |
| `HIVE_MINER`               | address       | if set, mining is enabled. value is coinbase   |
| `HIVE_MINER_EXTRA`         | hex           | extradata for mined blocks                     |
| `HIVE_CLIQUE_PERIOD`       | decimal       | enables clique PoA. value is target block time |
| `HIVE_CLIQUE_PRIVATEKEY`   | hex           | private key for signing of clique blocks       |
| `HIVE_SKIP_POW`            | 0 - 1         | disables PoW check during block import         |
| `HIVE_NETWORK_ID`          | decimal       | p2p network ID                                 |
| `HIVE_CHAIN_ID`            | decimal       | [EIP-155] chain ID                             |
| `HIVE_FORK_HOMESTEAD`      | decimal       | [Homestead][EIP-606] transition block          |
| `HIVE_FORK_DAO_BLOCK`      | decimal       | [DAO fork][EIP-779] transition block           |
| `HIVE_FORK_TANGERINE`      | decimal       | [Tangerine Whistle][EIP-608] transition block  |
| `HIVE_FORK_SPURIOUS`       | decimal       | [Spurious Dragon][EIP-607] transition block    |
| `HIVE_FORK_BYZANTIUM`      | decimal       | [Byzantium][EIP-609] transition block          |
| `HIVE_FORK_CONSTANTINOPLE` | decimal       | [Constantinople][EIP-1013] transition block    |
| `HIVE_FORK_PETERSBURG`     | decimal       | [Petersburg][EIP-1716] transition block        |
| `HIVE_FORK_ISTANBUL`       | decimal       | [Istanbul][EIP-1679] transition block          |
| `HIVE_FORK_MUIRGLACIER`    | decimal       | [Muir Glacier][EIP-2387] transition block      |
| `HIVE_FORK_BERLIN`         | decimal       | [Berlin][EIP-2070] transition block            |
| `HIVE_FORK_LONDON`         | decimal       | [London][london-spec] transition block         |

## LES client/server roles

Eth1 clients containing an implementation of [LES] may additionally support roles
`eth1_les_client` and `eth1_les_server`.

For the client role, the following additional variables should be supported:

| Variable                   | Value         |                                                |
|----------------------------|---------------|------------------------------------------------|
| `HIVE_NODETYPE`            | "light"       | enables LES client mode                        |

For the server role, the following additional variables should be supported:

| Variable                   | Value         |                                                |
|----------------------------|---------------|------------------------------------------------|
| `HIVE_LES_SERVER`          | 0 - 1         | if set to 1, LES server should be enabled      |


[LES]: https://github.com/ethereum/devp2p/blob/master/caps/les.md
[geth-docker]: ../clients/go-ethereum/Dockerfile
[hive-client-yaml]: ./commandline.md#client-build-parameters
[oe-genesis-jq]: ../clients/openethereum/mapper.jq
[EIP-155]: https://eips.ethereum.org/EIPS/eip-155
[EIP-606]: https://eips.ethereum.org/EIPS/eip-606
[EIP-607]: https://eips.ethereum.org/EIPS/eip-607
[EIP-608]: https://eips.ethereum.org/EIPS/eip-608
[EIP-609]: https://eips.ethereum.org/EIPS/eip-609
[EIP-779]: https://eips.ethereum.org/EIPS/eip-779
[EIP-1013]: https://eips.ethereum.org/EIPS/eip-1013
[EIP-1679]: https://eips.ethereum.org/EIPS/eip-1679
[EIP-1716]: https://eips.ethereum.org/EIPS/eip-1716
[EIP-2387]: https://eips.ethereum.org/EIPS/eip-2387
[EIP-2070]: https://eips.ethereum.org/EIPS/eip-2070
[london-spec]: https://github.com/ethereum/eth1.0-specs/blob/master/network-upgrades/mainnet-upgrades/london.md
[Overview]: ./overview.md
[Hive Commands]: ./commandline.md
[Simulators]: ./simulators.md
[Clients]: ./clients.md
