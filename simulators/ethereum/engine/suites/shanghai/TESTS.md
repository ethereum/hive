# `engine-shanghai` - Test Cases


Test Engine API shanghai, pre/post Shanghai: https://github.com/ethereum/execution-apis/blob/main/src/engine/shanghai.md

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/"
```

</details>

## Test Cases

### Withdrawals Fork on Block 2 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 2 (Paris) (Client)"
```

</details>

#### Description


Tests the transition to the withdrawals fork after a single block
has happened.
Block 1 is sent with invalid non-null withdrawals payload and
client is expected to respond with the appropriate error.


### Withdraw to a single account (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdraw to a single account (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to a single account.


### Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using sync
Sidechain reaches withdrawals fork at a lower block height
than the canonical chain


### GetPayloadBodiesByHash (Empty Transactions/Withdrawals) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodiesByHash (Empty Transactions/Withdrawals) (Paris) (Client)"
```

</details>

#### Description


Make no withdrawals and no transactions in many payloads.
Retrieve many of the payloads` bodies by hash.


### Withdrawals Fork On Genesis (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork On Genesis (Paris) (Client)"
```

</details>

#### Description


Tests the withdrawals fork happening since genesis (e.g. on a
testnet).


### GetPayloadV2 Block Value (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadV2 Block Value (Paris) (Client)"
```

</details>

#### Description


Verify the block value returned in GetPayloadV2.


### Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client
- Go through withdrawals fork on Block 2
- Withdraw to many accounts MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK times each block for 128 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account`s balance


### Withdrawals Fork on Block 8 - 10 Block Re-Org Sync (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 8 - 10 Block Re-Org Sync (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using sync
Re-org does not change withdrawals fork height, but changes
the payload at the height of the fork


### Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using NewPayload
Sidechain reaches withdrawals fork at a higher block height
than the canonical chain


### GetPayloadBodies Parallel (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodies Parallel (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each payload.
Retrieve many of the payloads` bodies by number range and hash in parallel, multiple times.


### Withdrawals Fork on Block 1 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 1 (Paris) (Client)"
```

</details>

#### Description


Tests the withdrawals fork happening directly after genesis.


### Withdraw to two accounts (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdraw to two accounts (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to two different accounts, repeated in
round-robin.
Reasoning: There might be a difference in implementation when an
account appears multiple times in the withdrawals list but the list
is not in ordered sequence.


### Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload (Paris) (Client)"
```

</details>

#### Description


Tests a 8 block re-org using NewPayload
Re-org does not change withdrawals fork height


### Withdrawals Fork on Block 8 - 10 Block Re-Org NewPayload (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 8 - 10 Block Re-Org NewPayload (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using NewPayload
Re-org does not change withdrawals fork height, but changes
the payload at the height of the fork


### Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using NewPayload
Sidechain reaches withdrawals fork at a lower block height
than the canonical chain


### Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync (Paris) (Client)"
```

</details>

#### Description


Tests a 10 block re-org using sync
Sidechain reaches withdrawals fork at a higher block height
than the canonical chain


### Withdraw zero amount (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdraw zero amount (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals where the amount withdrawn is 0.


### Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account - No Transactions (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account - No Transactions (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client
- Go through withdrawals fork on Block 1
- Withdraw to a single account MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK times each block for 2 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync and verify withdrawn account`s balance


### Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client
- Go through withdrawals fork on Block 1
- Withdraw to a single account MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK times each block for 2 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync and verify withdrawn account`s balance


### Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts - No Transactions (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts - No Transactions (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client
- Go through withdrawals fork on Block 2
- Withdraw to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each block for 2 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account`s balance


### GetPayloadBodies After Sync (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodies After Sync (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each payload.
Spawn a secondary client which must sync the canonical chain
from the first client.
Retrieve many of the payloads` bodies by number range from
this secondary client.


### Withdraw many accounts (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdraw many accounts (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK * 5 different accounts.
Execute many blocks this way.


### GetPayloadBodiesByRange (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodiesByRange (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each payload.
Retrieve many of the payloads` bodies by number range.


### GetPayloadBodiesByRange (Empty Transactions/Withdrawals) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodiesByRange (Empty Transactions/Withdrawals) (Paris) (Client)"
```

</details>

#### Description


Make no withdrawals and no transactions in many payloads.
Retrieve many of the payloads` bodies by number range.


### GetPayloadBodiesByHash (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodiesByHash (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each payload.
Retrieve many of the payloads` bodies by hash.


### Withdrawals Fork on Block 3 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 3 (Paris) (Client)"
```

</details>

#### Description


Tests the transition to the withdrawals fork after two blocks
have happened.
Block 2 is sent with invalid non-null withdrawals payload and
client is expected to respond with the appropriate error.


### Corrupted Block Hash Payload (INVALID) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Corrupted Block Hash Payload (INVALID) (Paris) (Client)"
```

</details>

#### Description


Send a valid payload with a corrupted hash using engine_newPayloadV2.


### Sync after 2 blocks - Withdrawals on Genesis - Single Withdrawal Account (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 2 blocks - Withdrawals on Genesis - Single Withdrawal Account (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client, with Withdrawals since genesis
- Withdraw to a single account MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK times each block for 2 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync and verify withdrawn account`s balance


### GetPayloadBodiesByRange (Sidechain) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/GetPayloadBodiesByRange (Sidechain) (Paris) (Client)"
```

</details>

#### Description


Make multiple withdrawals to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each payload.
Retrieve many of the payloads` bodies by number range.
Create a sidechain extending beyond the canonical chain block number.


### Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts (Paris) (Client)"
```

</details>

#### Description


- Spawn a first client
- Go through withdrawals fork on Block 2
- Withdraw to MAINNET_MAX_WITHDRAWAL_COUNT_PER_BLOCK accounts each block for 2 blocks
- Spawn a secondary client and send FCUV2(head)
- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account`s balance


### Empty Withdrawals (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Empty Withdrawals (Paris) (Client)"
```

</details>

#### Description


Produce withdrawals block with zero withdrawals.


### Withdrawals Fork on Block 1 - 1 Block Re-Org (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 1 - 1 Block Re-Org (Paris) (Client)"
```

</details>

#### Description


Tests a simple 1 block re-org 


### Withdrawals Fork on Block 1 - 8 Block Re-Org, Sync (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-shanghai/Withdrawals Fork on Block 1 - 8 Block Re-Org, Sync (Paris) (Client)"
```

</details>

#### Description


Tests a 8 block re-org using NewPayload
Re-org does not change withdrawals fork height


