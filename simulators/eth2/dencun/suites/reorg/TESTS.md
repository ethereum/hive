# Deneb Reorg - Test Cases

Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb and test re-orgs of the beacon chain.

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-reorg/"
```

</details>

## Test Cases

### - test-reorg-from-capella-

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-reorg/test-reorg-from-capella-"
```

</details>

#### Description


Test re-org of the beacon chain by a third non-validating client, re-org from capella.
Start two clients disconnected from each other, then connect them through a third client and check that they re-org.


#### Testnet Configuration


- Node Count: 5
- Validating Node Count: 5
- Validator Key Count: 128
- Validator Key per Node: 25- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 0
- BLS Withdrawal Credentials Count: 128

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

### - test-reorg-from-deneb-

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-reorg/test-reorg-from-deneb-"
```

</details>

#### Description


Test re-org of the beacon chain by a third non-validating client, re-org from deneb.
Start two clients disconnected from each other, then connect them through a third client and check that they re-org.


#### Testnet Configuration


- Node Count: 5
- Validating Node Count: 5
- Validator Key Count: 128
- Validator Key per Node: 25- Genesis Fork: Deneb
- Execution Withdrawal Credentials Count: 0
- BLS Withdrawal Credentials Count: 128

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

