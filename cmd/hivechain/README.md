# hivechain

Hivechain creates a non-empty blockchain for testing purposes. To facilitate good tests,
the created chain excercises many protocol features, including:

- different types of transactions
- diverse set of contracts with interesting storage, code, etc.
- contracts to create known log events
- non-transaction chain modifications: coinbase fee payments, uncles, withdrawals...

## Running hivechain

Here is an example command line invocation of the tool:

    hivechain generate -fork-interval 6 -tx-interval 1 -length 500 -outdir chain -outputs genesis,chain,headfcu

The command creates a 500-block chain where a new fork gets enabled every six blocks, and
every block contains one 'modification' (i.e. transaction). A number of output files will
be created in the `chain/` directory:

- `genesis.json` contains the genesis block specification
- `chain.rlp` has the blocks in binary RLP format
- `headfcu.json` contains an Engine API request that sends the head block to a client

To see all generator options, run:

    hivechain generate -help

## -outputs

Different kinds of output files can be created based on the generated chain. The available
output formats are documented below.

### accounts

Creates `accounts.json` containing accounts and corresponding private keys.

### chain, powchain

`chain` creates `chain.rlp` containing the chain blocks.

`powchain` creates `powchain.rlp` containing only the pre-merge blocks.

### fcu, headfcu, newpayload

`fcu.json` is a JSON array of forkchoiceUpdated requests for all post-merge blocks.

`headfcu.json` is a request for just the head block. This is useful for triggering a sync in the client.

`newpayload.json` is a JSON array of newPayload requests for post-merge blocks.

### genesis

This writes the `genesis.json` file containing a go-ethereum style genesis spec. Note
this file includes the fork block numbers/timestamps.

### headblock

This creates `headblock.json` with a dump of the head header.

### headstate

This writes `headstate.json`, a dump of the complete state of the head block.

### txinfo

The `txinfo.json` file contains an object with a key for each block modifier, and the
value being information about the activity of the modifier.
