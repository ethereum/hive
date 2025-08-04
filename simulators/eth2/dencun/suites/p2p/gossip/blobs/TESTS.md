# Deneb P2P Blobs Gossip - Test Cases

Collection of test vectors that verify client behavior under different blob gossiping scenarios.

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/"
```

</details>

## Test Cases

### - Blob Gossiping Sanity

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/blob-gossiping-sanity-"
```

</details>

#### Description


Sanity test where the blobber is verified to be working correctly


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Sign the block
- Generate the blob sidecars using signed header
- Broadcast the block
- Broadcast the blob sidecars

### - Blob Gossiping Before Block

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/blob-gossiping-before-block-"
```

</details>

#### Description


Test chain health where the blobs are gossiped before the block


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Sign the block
- Generate the blob sidecars using signed header
- Broadcast the blob sidecars
- Broadcast the block

### - Blob Gossiping Delay

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/blob-gossiping-delay-"
```

</details>

#### Description


Test chain health where the blobs are gossiped after the block with a 500ms delay


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Sign the block
- Generate the blob sidecars using signed header
- Broadcast the block
- Insert a delay of 500 milliseconds
- Broadcast the blob sidecars

### - Blob Gossiping One-Slot Delay

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/blob-gossiping-one-slot-delay-"
```

</details>

#### Description


Test chain health where the blobs are gossiped after the block with a 6s delay


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Sign the block
- Generate the blob sidecars using signed header
- Broadcast the block
- Insert a delay of 6000 milliseconds
- Broadcast the blob sidecars

### - Invalid Equivocating Block

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/invalid-equivocating-block-"
```

</details>

#### Description


Test chain health if a proposer sends an invalid equivocating block and the correct block
at the same time to different peers.

Blob sidecars contain the correct block header.

Slot action is executed every other slot because, although it does not cause a missed slot,
clients might reject the p2p block message due to it being a slashable offense, so this
delay makes the test more deterministic.


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Create an invalid equivocating block by modifying the graffiti
- Sign both blocks
- Generate the sidecars out of the correct block only
- Broadcast the blob sidecars
- Broadcast the equivocating signed block and the correct signed block to different peers

### - Equivocating Block and Blobs

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/equivocating-block-and-blobs-"
```

</details>

#### Description


Test chain health if a proposer sends equivocating blobs and block to different peers


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Create an equivocating block by modifying the graffiti
- Sign both blocks
- Generate blob sidecars for both blocks
- Broadcast the blob sidecars for both blocks to different peers
- Broadcast the signed blocks to different peers

### - Equivocating Block Header in Blob Sidecars

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/equivocating-block-header-in-blob-sidecars-"
```

</details>

#### Description


Test chain health if a proposer sends equivocating blob sidecars (equivocating block header), but the correct full block is sent first.


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Create an invalid equivocating block by modifying the graffiti
- Sign both blocks
- Generate the sidecars out of the equivocating signed block only
- Broadcast the original signed block only
- Broadcast the blob sidecars with the equivocating block header

### - Equivocating Block Header in Blob Sidecars 2

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/equivocating-block-header-in-blob-sidecars-2-"
```

</details>

#### Description


Test chain health if a proposer sends equivocating blob sidecars (equivocating block header), and the correct full block is sent afterwards.


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Create an invalid equivocating block by modifying the graffiti
- Sign both blocks
- Generate the sidecars out of the equivocating signed block only
- Broadcast the blob sidecars with the equivocating block header
- Broadcast the original signed block only

### - Equivocating Blob Sidecars

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-p2p-blobs-gossip/equivocating-blobs-"
```

</details>

#### Description


Test chain health if a proposer sends equivocating blob sidecars (equivocating block header) to a set of peers, and the correct blob sidecars to another set of peers. The correct block is sent to all peers afterwards.


#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

#### Blobber Behavior


- Create an equivocating block by modifying the graffiti
- Sign both blocks
- Generate blob sidecar bundles out of both signed blocks
- Broadcast both blob sidecar bundles to different peers
- Broadcast the original signed block only

