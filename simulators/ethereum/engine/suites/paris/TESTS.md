# `engine-paris` - Test Cases


Test Engine API tests using CL mocker to inject commands into clients after they 
have reached the Terminal Total Difficulty: https://github.com/ethereum/execution-apis/blob/main/src/engine/paris.md

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/"
```

</details>

## Test Case Categories

- [Engine API: NewPayload](#category-engine-api-newpayload)

- [Other](#category-other)

- [Fork ID](#category-fork-id)

## Category: Engine API: NewPayload

### Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=true) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=true) (Paris) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a ForkchoiceUpdated to set the head to an unknown head hash, to set the client in SYNCING state
- Send a NewPayload with an invalid hash and a known parent hash
- Verify that the client rejects the payload

### Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=false) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=false) (Paris) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and an unknown parent hash
- Verify that the client rejects the payload

### ParentHash equals BlockHash on NewPayload, Syncing=True (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/ParentHash equals BlockHash on NewPayload, Syncing=True (Paris) (Client)"
```

</details>

#### Description


Incorrectly set the parent hash into the block hash of an otherwise valid payload:
- Produce 5 valid blocks

- Modify next payload to set the blockHash set to the same value as the parentHash and send using NewPayload, where parentHash is an unknown block

### ParentHash equals BlockHash on NewPayload, Syncing=False (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/ParentHash equals BlockHash on NewPayload, Syncing=False (Paris) (Client)"
```

</details>

#### Description


Incorrectly set the parent hash into the block hash of an otherwise valid payload:
- Produce 5 valid blocks

- Modify next payload to set the blockHash set to the same value as the parentHash and send using NewPayload, where parentHash is the head of the canonical chain

### Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=true) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=true) (Paris) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and a parent hash pointing to a known ancestor of the canonical head
- Verify that the client rejects the payload

### Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=false) (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=false) (Paris) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and a parent hash pointing to the canonical head
- Verify that the client rejects the payload

## Category: Fork ID

### Fork ID: Genesis=1, Paris=1, BlocksBeforePeering=1 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=1, Paris=1, BlocksBeforePeering=1 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- London Time: 0
	- Paris Time: 1

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Paris=0, BlocksBeforePeering=1 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=1, Paris=0, BlocksBeforePeering=1 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- London Time: 0
	- Paris Time: 0

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Paris=1 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=1, Paris=1 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- London Time: 0
	- Paris Time: 1

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Paris=0, BlocksBeforePeering=1 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=0, Paris=0, BlocksBeforePeering=1 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- London Time: 0
	- Paris Time: 0

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Paris=0 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=1, Paris=0 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- London Time: 0
	- Paris Time: 0

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Paris=0 (Paris) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-paris/Fork ID: Genesis=0, Paris=0 (Paris) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- London Time: 0
	- Paris Time: 0

- Peer with the client and verify the Fork ID is correct


