# Deneb Testnet - Test Cases

Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb.

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-testnet/"
```

</details>

## Test Cases

### - Deneb Fork

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-testnet/test-deneb-fork-"
```

</details>

#### Description

Sanity test to check the fork transition to deneb.

#### Testnet Configuration


- Node Count: 2
- Validating Node Count: 2
- Validator Key Count: 128
- Validator Key per Node: 64
- Genesis Fork: Capella
- Execution Withdrawal Credentials Count: 128
- BLS Withdrawal Credentials Count: 0

#### Verifications (Execution Client)


- Blob (type-3) transactions are included in the blocks

#### Verifications (Consensus Client)


- For each blob transaction on the execution chain, the blob sidecars are available for the beacon block at the same height
- The beacon block lists the correct commitments for each blob

### - Deneb Genesis

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim eth2/dencun --sim.limit "eth2-deneb-testnet/test-deneb-genesis-"
```

</details>

#### Description


Sanity test to check the beacon clients can start with deneb genesis.


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
- After all other verifications are done, the beacon chain is able to finalize the current epoch

