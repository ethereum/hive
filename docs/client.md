## Hive Ethereum Client Interface

TODO: document client branch name

This page explains how client containers work in Hive.

The client definitions live in the `clients` folder, inside a folder named `<project>`
where `<project>` is the official name of the client (lowercase, no fancy characters).
`hive` will automatically pick up all clients from this folder.

Adding a new client implementation to `hive` entails creating a Dockerfile (and related
resources), based on which `hive` will assemble the docker image to use as the blueprint
for testing.

There aren't many contraints on the image itself, though a few required caveats exist:

 * It should be as tiny as possible (play nice with others). Preferably use `alpine` Linux.
 * It should expose the following ports: 8545 (HTTP RPC), 8546 (WS RPC), 30303 (devp2p).
 * It should have a single entrypoint (or script) defined, which can initialize and run the client.

For guidance, check out the reference [go-ethereum](../clients/go-ethereum/Dockerfile) client.

## Initializing the client

hive injects all the required configurations into the Linux containers prior to launching
the client's `entrypoint` script. It is then left to this script to interpret all the
environmental configs and initialize the client appropriately.

The chain configurations files:

 * `/genesis.json` contains the JSON specification of the Ethereum genesis states
 * `/chain.rlp` contains a batch of RLP encoded blocks to import before startup
 * `/blocks/` folder with numbered singleton blocks to import before startup
 * `/keys/` contains account keys that should be imported before startup

Client startup scripts need to ensure that they load the genesis state first, then import
a possibly longer blockchain and then import possibly numerous individual blocks. The
reason for requiring two different block sources is that specifying a single chain is more
optimal, but tests requiring forking chains cannot create a single chain.

Besides the standardized chain configurations, clients can in general be modified
behavior-wise in quite a few ways that are mostly supported by all clients, yet are
implemented differently in each. As such, each possible behavioral change required by some
simulator is characterized by an environment variable, which clients should interpret as
best as they can.

The behavioral configuration variables:

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

The client has the responsibility of mapping the hive environment variables to its own
command line flags. To assist in this, Hive illustrates a technique in the
`clients/go-ethereum` folder using `mapper.jq`, which is invoked in `geth.sh` This
technique can be replicated for other clients.

## Enode script

For devp2p tests or other simulations that require to know the specific enode URL of the
client instance, the client must provide an `enode.sh` that echoes the enode of the
running instance. This is executed by the Hive host remotely in order to retrieve the
enode URL.

## Starting the client

After initializing the client blockchain (genesis, chain, blocks), the last task of the
entry script is to start up the client itself. The following defaults are required by
`hive` to enable automatic network assembly and firewall enforcement:

 * Clients should open their HTTP-RPC endpoint on `0.0.0.0:8545` (mandatory)
 * Clients should open their WS-RPC endpoint on `0.0.0.0:8546` (optional)
 * Clients should open their IPC-RPC endpoints at `/rpc.ipc` (optional)

There is no need to handle graceful client termination. Clients will be forcefully aborted
upon test suite completion and all related data purged. A new instance will be started for
every test.

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
