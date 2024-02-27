# Deneb Sync - Test Cases

Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb and test syncing of the beacon chain.

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-sync/"
```

</details>

## Test Cases

### - Sync From Capella Transition

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-sync/test-sync-from-capella-"
```

</details>

#### Description


Test syncing of the beacon chain by a secondary non-validating client, sync from capella.

- Start two validating nodes that begin on Capella/Shanghai genesis
- Deneb/Cancun transition occurs on Epoch 1
- Total of 128 Validators, 64 for each validating node
- Wait for Deneb fork and start sending blob transactions to the Execution client
- Wait one more epoch for the chain to progress and include blobs
- Start a third client with the first two validating nodes as bootnodes
- Wait for the third client to reach the head of the canonical chain
- Verify on the consensus client on the synced client that:
  - For each blob transaction on the execution chain, the blob sidecars are available for the
	beacon block at the same height
  - The beacon block lists the correct commitments for each blob
  - The blob sidecars and kzg commitments match on each block for the synced client


#### Testnet Configuration


- Node Count: 3
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 0
- BLS Withdrawal Credentials Count: 128

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

### - Sync From Deneb Genesis

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-sync/test-sync-from-deneb-"
```

</details>

#### Description


Test syncing of the beacon chain by a secondary non-validating client, sync from deneb.

- Start two validating nodes that begin on Deneb genesis
- Total of 128 Validators, 64 for each validating node
- Start sending blob transactions to the Execution client
- Wait one epoch for the chain to progress and include blobs
- Start a third client with the first two validating nodes as bootnodes
- Wait for the third client to reach the head of the canonical chain
- Verify on the consensus client on the synced client that:
  - For each blob transaction on the execution chain, the blob sidecars are available for the
	beacon block at the same height
  - The beacon block lists the correct commitments for each blob
  - The blob sidecars and kzg commitments match on each block for the synced client


#### Testnet Configuration


- Node Count: 3
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 0
- BLS Withdrawal Credentials Count: 128

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

