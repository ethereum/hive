## Hive Ethereum Client Interface

This page explains how client containers work in Hive.

Clients are docker images which can be instantiated by a simulation. A client definition
consist of a Dockerfile and associated resources. Client definitions live in
subdirectories of `clients/` in the hive repository.

When hive runs a simulation, it first builds all client docker images using their
Dockerfile, i.e. it basically runs `docker build .` in the client directory. Since most
client definitions wrap an existing Ethereum client, and building the client from source
may take a long time, it is usually best to base the hive client on a pre-built docker
image from Docker Hub.

Client Dockerfiles should support an optional argument named `branch`, which specifies the
requested client version. This argument can be set by users by appending it to the client
name like:

    ./hive --sim my-simulation --client go-ethereum_v1.9.23,go_ethereum_v1.9.22

See the [go-ethereum](../clients/go-ethereum/Dockerfile) client definition for an example
of a client Dockerfile.

## Client Lifecycle

When the simulation requests a client instance, hive creates a docker container from the
client image. The simulator can customize the container by passing environment variables
with prefix `HIVE_`. It may also upload files into the container before it starts. Once
the container is created, hive simply runs the entry point defined in the `Dockerfile`.

For all client containers, hive waits for TCP port 8545 to open before considering the
client ready for use by the simulator. If the client container does not open this port
within a certain timeout, hive assumes the client has failed to start.

Environment variables and files interpreted by thef entry point define a 'protocol'
between the simulator and client. While hive itself does not require support for any
specific variables or files, simulators usually expect client containers to be
configurable in certain ways. In order to run tests against multiple Ethereum clients, for
example, the simulator needs to be able to configure all clients for a specific blockchain
and make them join the peer-to-peer network used for testing.

## Eth1 Client Requirements

This section describes the requirements for Ethereum 1 client wrappers in hive. Client
entry point scripts must support this interface in order to be tested by existing Ethereum
1.x-specific simulators.

Clients must provide JSON-RPC over HTTP on TCP port 8545. They may also support JSON-RPC
over WebSocket on port 8546, but this is not strictly required.

### Files

The simulator may customize client start by placing these files into the client container:

- `/genesis.json` contains Ethereum genesis state in the JSON format used by Geth. This
  file is mandatory.
- `/chain.rlp` contains RLP-encoded blocks to import before startup.
- `/blocks/` directory containg `.rlp` files.

On startup, client entry point scripts must first load the genesis block and state into
the client implementation from `/genesis.json`. To do this, the script needs to translate
from Geth genesis format into a format appropriate for the specific client implementation.
The translation is usually done using a jq script. See the [openethereum genesis
translator][oe-genesis-jq], for example.

After the genesis state, the client should import the blocks from `/chain.rlp` if it is
present, and finally import the individual blocks from `/blocks` in file name order. The
reason for requiring two different block sources is that specifying a single chain is more
optimal, but tests requiring forking chains cannot create a single chain. The client
should start even if the blocks are invalid, i.e. after the import, the client's 'best
block' should be the last valid, imported block.

### Environment

Clients must support the following environment variables. The client's entry point script
may map these to command line flags or use them generate a config file, for example.

| Variable                   | Value                |                                                |
|----------------------------|----------------------|------------------------------------------------|
| `HIVE_LOGLEVEL`            | 0 - 5                | configures log level of client                 |
| `HIVE_NODETYPE`            | archive, full, light | sets sync algorithm                            |
| `HIVE_BOOTNODE`            | enode URL            | makes client connect to another node           |
| `HIVE_GRAPHQL_ENABLED`     | 0 - 1                | if set, GraphQL is enabled on port 8545        |
| `HIVE_MINER`               | address              | if set, mining is enabled. value is coinbase   |
| `HIVE_MINER_EXTRA`         | hex                  | extradata for mined blocks                     |
| `HIVE_CLIQUE_PERIOD`       | decimal              | enables clique PoA. value is target block time |
| `HIVE_CLIQUE_PRIVATEKEY`   | hex                  | private key for signing of clique blocks       |
| `HIVE_SKIP_POW`            | 0 - 1                | disables PoW check during block import         |
| `HIVE_NETWORK_ID`          | decimal              | p2p network ID                                 |
| `HIVE_CHAIN_ID`            | decimal              | [EIP-155] chain ID                             |
| `HIVE_FORK_HOMESTEAD`      | decimal              | [Homestead][EIP-606] transition block          |
| `HIVE_FORK_DAO_BLOCK`      | decimal              | [DAO fork][EIP-779] transition block           |
| `HIVE_FORK_TANGERINE`      | decimal              | [Tangerine Whistle][EIP-608] transition block  |
| `HIVE_FORK_SPURIOUS`       | decimal              | [Spurious Dragon][EIP-607] transition block    |
| `HIVE_FORK_BYZANTIUM`      | decimal              | [Byzantium][EIP-609] transition block          |
| `HIVE_FORK_CONSTANTINOPLE` | decimal              | [Constantinople][EIP-1013] transition block    |
| `HIVE_FORK_PETERSBURG`     | decimal              | [Petersburg][EIP-1716] transition block        |
| `HIVE_FORK_ISTANBUL`       | decimal              | [Istanbul][EIP-1679] transition block          |
| `HIVE_FORK_MUIRGLACIER`    | decimal              | [Muir Glacier][EIP-2387] transition block      |
| `HIVE_FORK_BERLIN`         | decimal              | [Berlin][EIP-2070] transition block            |

### Enode script

Some tests require peer-to-peer node information of the client instance. The client
container must contain an `/enode.sh` script that echoes the enode of the running
instance. This script is executed by Hive host in order to retrieve the enode URL.

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
[oe-genesis-jq]: ../clients/openethereum/mapper.jq
