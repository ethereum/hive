# Deneb Builder - Test Cases


Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet and builder API for Cancun+Deneb.


### Clients that require finalization to enable builder
- Lighthouse
- Teku


## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/"
```

</details>

## Test Cases

### - Deneb Builder Workflow From Capella Transition

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-sanity-"
```

</details>

#### Description


Test canonical chain includes deneb payloads built by the builder api.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 0
- BLS Withdrawal Credentials Count: 128
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder must be able to include blocks with blobs in the canonical chain, which implicitly verifies:
	- Consensus client is able to properly format header requests to the builder
	- Consensus client is able to properly format blinded signed requests to the builder
	- No signed block contained an invalid format or signature
	- Test fails with a timeout if no payload with blobs is produced after the fork

### - Deneb Builder Builds Block With Invalid Beacon Root, Correct State Root

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-invalid-payload-attributes-beacon-root-"
```

</details>

#### Description


Test canonical chain can still finalize if the builders start
building payloads with invalid parent beacon block root.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder starts producing invalid payloads, verify that:
	- None of the produced payloads are included in the canonical chain
- Since action causes missed slot, verify that the circuit breaker correctly kicks in and disables the builder workflow. Builder starts corrupting payloads after fork, hence a single block in the canonical chain after the fork is enough to verify the circuit breaker

### - Deneb Builder Errors Out on Header Requests After Deneb Transition

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-error-on-deneb-header-request-"
```

</details>

#### Description


Test canonical chain can still finalize if the builders start
returning error on header request after deneb transition.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder starts producing invalid payloads, verify that:
	- None of the produced payloads are included in the canonical chain

### - Deneb Builder Errors Out on Signed Blinded Beacon Block/Blob Sidecars Submission After Deneb Transition

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-error-on-deneb-unblind-payload-request-"
```

</details>

#### Description


Test canonical chain can still finalize if the builders start
returning error on unblinded payload request after deneb transition.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder starts producing invalid payloads, verify that:
	- None of the produced payloads are included in the canonical chain
- Since action causes missed slot, verify that the circuit breaker correctly kicks in and disables the builder workflow. Builder starts corrupting payloads after fork, hence a single block in the canonical chain after the fork is enough to verify the circuit breaker

### - Deneb Builder Builds Block With Invalid Payload Version

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-invalid-payload-version-"
```

</details>

#### Description


Test consensus clients correctly reject a built payload if the
version is outdated (capella instead of deneb).

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder starts producing invalid payloads, verify that:
	- None of the produced payloads are included in the canonical chain

### - Deneb Builder Builds Block With Invalid Beacon Root, Incorrect State Root

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-builder/test-builders-invalid-payload-beacon-root-"
```

</details>

#### Description


Test consensus clients correctly circuit break builder after a
period of empty blocks due to invalid unblinded blocks.

The payloads are built using an invalid parent beacon block root, which can only
be caught after unblinding the entire payload and running it in the
local execution client, at which point another payload cannot be
produced locally and results in an empty slot.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0
- Deneb/Cancun transition occurs on Epoch 1 or 5
	- Epoch depends on whether builder workflow activation requires finalization [on the CL client](#clients-that-require-finalization-to-enable-builder).
- Builder is enabled for all nodes
- Builder action is only enabled after fork
- Nodes have the mock-builder configured as builder endpoint

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob
- Verify that the builder, up to before Deneb fork, has been able to produce blocks and they have been included in the canonical chain
- After Deneb fork, the builder starts producing invalid payloads, verify that:
	- None of the produced payloads are included in the canonical chain
- Since action causes missed slot, verify that the circuit breaker correctly kicks in and disables the builder workflow. Builder starts corrupting payloads after fork, hence a single block in the canonical chain after the fork is enough to verify the circuit breaker

