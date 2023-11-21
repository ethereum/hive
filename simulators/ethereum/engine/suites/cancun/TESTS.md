# `engine-cancun` - Test Cases


Test Engine API on Cancun: https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md

## Run Suite

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/"
```

</details>

## Test Case Categories

- [Fork ID](#category-fork-id)

- [Cancun: NewPayloadV3 Versioned Hashes](#category-cancun-newpayloadv3-versioned-hashes)

- [Cancun: NewPayloadV3](#category-cancun-newpayloadv3)

- [Engine API: NewPayload](#category-engine-api-newpayload)

- [Cancun: Blob Transactions](#category-cancun-blob-transactions)

- [Cancun: Fork Timestamp](#category-cancun-fork-timestamp)

- [Other](#category-other)

- [Cancun: GetPayloadV3](#category-cancun-getpayloadv3)

- [Cancun: BlobGasUsed](#category-cancun-blobgasused)

- [Cancun: Blob Transactions DevP2P](#category-cancun-blob-transactions-devp2p)

- [Cancun: ForkchoiceUpdatedV3](#category-cancun-forkchoiceupdatedv3)

## Category: Fork ID

### Fork ID: Genesis=1, Cancun=0 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=0 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 0

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2, Shanghai=2 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2, Shanghai=2 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 2
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 1
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=0, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=0, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 0

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2, Shanghai=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2, Shanghai=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 1
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2, Shanghai=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2, Shanghai=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 1
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2, Shanghai=2, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2, Shanghai=2, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 2
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=0 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=0 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 0

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=1, Shanghai=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=1, Shanghai=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 1
	- Cancun Time: 1

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=1, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=1, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 1
	- Cancun Time: 1

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=1, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=1, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 1
	- Cancun Time: 1

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=2, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=2, Shanghai=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 1
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 1

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2, Shanghai=2, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2, Shanghai=2, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 2
	- Cancun Time: 2

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=0, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=0, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 0

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=2, Shanghai=2 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=2, Shanghai=2 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 2
	- Cancun Time: 2

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=1, BlocksBeforePeering=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=1, BlocksBeforePeering=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 1

- Produce 1 blocks
- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 0
	- Cancun Time: 1

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=1, Cancun=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=1, Cancun=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 1
	- Shanghai Time: 0
	- Cancun Time: 1

- Peer with the client and verify the Fork ID is correct


### Fork ID: Genesis=0, Cancun=1, Shanghai=1 (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Fork ID: Genesis=0, Cancun=1, Shanghai=1 (Cancun) (Client)"
```

</details>

#### Description


- Start a client with the following genesis configuration:
	- Genesis Timestamp: 0
	- Shanghai Time: 1
	- Cancun Time: 1

- Peer with the client and verify the Fork ID is correct


## Category: Cancun: NewPayloadV3 Versioned Hashes

### NewPayloadV3 Versioned Hashes, Extra Hash, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Extra Hash, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is has an extra hash for a blob that is not in the payload.


### NewPayloadV3 Versioned Hashes, Empty Hashes, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Empty Hashes, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is empty, even though there are blobs in the payload.


### NewPayloadV3 Versioned Hashes, Non-Empty Hashes, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Non-Empty Hashes, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is contains hashes, even though there are no blobs in the payload.


### NewPayloadV3 Versioned Hashes, Missing Hash, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Missing Hash, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is missing one of the hashes.


### NewPayloadV3 Versioned Hashes, Repeated Hash, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Repeated Hash, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a blob that is repeated in the array.


### NewPayloadV3 Versioned Hashes, Nil Hashes, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Nil Hashes, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is nil, even though the fork has already happened.


### NewPayloadV3 Versioned Hashes, Incorrect Version, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Incorrect Version, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a single blob that has an incorrect version.


### NewPayloadV3 Versioned Hashes, Incorrect Hash, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Incorrect Hash, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a blob hash that does not belong to any blob contained in the payload.


### NewPayloadV3 Versioned Hashes, Incorrect Version, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Incorrect Version, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a single blob that has an incorrect version.


### NewPayloadV3 Versioned Hashes, Missing Hash, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Missing Hash, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is missing one of the hashes.


### NewPayloadV3 Versioned Hashes, Non-Empty Hashes, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Non-Empty Hashes, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is contains hashes, even though there are no blobs in the payload.


### NewPayloadV3 Versioned Hashes, Out of Order, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Out of Order, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is out of order.


### NewPayloadV3 Versioned Hashes, Out of Order, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Out of Order, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is out of order.


### NewPayloadV3 Versioned Hashes, Repeated Hash, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Repeated Hash, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a blob that is repeated in the array.


### NewPayloadV3 Versioned Hashes, Nil Hashes, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Nil Hashes, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is nil, even though the fork has already happened.


### NewPayloadV3 Versioned Hashes, Incorrect Hash, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Incorrect Hash, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
has a blob that is repeated in the array.


### NewPayloadV3 Versioned Hashes, Empty Hashes, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Empty Hashes, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is empty, even though there are blobs in the payload.


### NewPayloadV3 Versioned Hashes, Extra Hash, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Versioned Hashes, Extra Hash, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Tests VersionedHashes in Engine API NewPayloadV3 where the array
is has an extra hash for a blob that is not in the payload.


## Category: Cancun: NewPayloadV3

### NewPayloadV3 After Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Empty Array Versioned Hashes (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 After Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Empty Array Versioned Hashes (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 After Cancun with:
- 0x00 ExcessBlobGas
- nil BlobGasUsed
- Empty Versioned Hashes Array


### NewPayloadV3 Before Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- 0x00 ExcessBlobGas
- nil BlobGasUsed
- nil Versioned Hashes Array
- nil Beacon Root


### NewPayloadV3 After Cancun, 0x00 Blob Fields, Empty Array Versioned Hashes, Nil Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 After Cancun, 0x00 Blob Fields, Empty Array Versioned Hashes, Nil Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 After Cancun with:
- 0x00 ExcessBlobGas
- nil BlobGasUsed
- Empty Versioned Hashes Array


### NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Zero Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Zero Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- nil ExcessBlobGas
- nil BlobGasUsed
- nil Versioned Hashes Array
- Zero Beacon Root


### NewPayloadV3 Before Cancun, 0x00 Data Fields, Empty Array Versioned Hashes, Zero Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, 0x00 Data Fields, Empty Array Versioned Hashes, Zero Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- 0x00 ExcessBlobGas
- 0x00 BlobGasUsed
- Empty Versioned Hashes Array
- Zero Beacon Root


### NewPayloadV3 Before Cancun, Nil Data Fields, Empty Array Versioned Hashes, Nil Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, Nil Data Fields, Empty Array Versioned Hashes, Nil Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- nil ExcessBlobGas
- nil BlobGasUsed
- Empty Versioned Hashes Array
- nil Beacon Root


### NewPayloadV3 After Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Empty Array Versioned Hashes, Zero Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 After Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Empty Array Versioned Hashes, Zero Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 After Cancun with:
- nil ExcessBlobGas
- 0x00 BlobGasUsed
- Empty Versioned Hashes Array
- Zero Beacon Root


### NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- nil ExcessBlobGas
- nil BlobGasUsed
- nil Versioned Hashes Array
- nil Beacon Root

Verify that client returns INVALID_PARAMS_ERROR


### NewPayloadV3 Before Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/NewPayloadV3 Before Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending NewPayloadV3 Before Cancun with:
- nil ExcessBlobGas
- 0x00 BlobGasUsed
- nil Versioned Hashes Array
- nil Beacon Root


## Category: Engine API: NewPayload

### Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=true) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=true) (Cancun) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a ForkchoiceUpdated to set the head to an unknown head hash, to set the client in SYNCING state
- Send a NewPayload with an invalid hash and a known parent hash
- Verify that the client rejects the payload

### ParentHash equals BlockHash on NewPayload, Syncing=False (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ParentHash equals BlockHash on NewPayload, Syncing=False (Cancun) (Client)"
```

</details>

#### Description


Incorrectly set the parent hash into the block hash of an otherwise valid payload:
- Produce 5 valid blocks

- Modify next payload to set the blockHash set to the same value as the parentHash and send using NewPayload, where parentHash is the head of the canonical chain

### ParentHash equals BlockHash on NewPayload, Syncing=True (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ParentHash equals BlockHash on NewPayload, Syncing=True (Cancun) (Client)"
```

</details>

#### Description


Incorrectly set the parent hash into the block hash of an otherwise valid payload:
- Produce 5 valid blocks

- Modify next payload to set the blockHash set to the same value as the parentHash and send using NewPayload, where parentHash is an unknown block

### Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=false) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=false) (Cancun) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and a parent hash pointing to the canonical head
- Verify that the client rejects the payload

### Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=false) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlockHash on NewPayload (Syncing=true, Sidechain=false) (Cancun) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and an unknown parent hash
- Verify that the client rejects the payload

### Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=true) (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlockHash on NewPayload (Syncing=false, Sidechain=true) (Cancun) (Client)"
```

</details>

#### Description


Corrupt the hash of an otherwise valid payload:
- Produce 5 valid blocks
- Send a NewPayload with an invalid hash and a parent hash pointing to a known ancestor of the canonical head
- Verify that the client rejects the payload

## Category: Cancun: Blob Transactions

### Blob Transaction Ordering, Multiple Accounts (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transaction Ordering, Multiple Accounts (Cancun) (Client)"
```

</details>

#### Description


Send N blob transactions with `cancun.MAX_BLOBS_PER_BLOCK-1` blobs each,
using account A.
Send N blob transactions with 1 blob each from account B.
Verify that the payloads are created with the correct ordering:
 - All payloads must have full blobs.
All transactions have sufficient data gas price to be included any
of the payloads.


### Blob Transactions On Block 1, Cancun Genesis (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transactions On Block 1, Cancun Genesis (Cancun) (Client)"
```

</details>

#### Description


Tests the Cancun fork since genesis.

Verifications performed:
* See Blob Transactions On Block 1, Shanghai Genesis


### Replace Blob Transactions (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Replace Blob Transactions (Cancun) (Client)"
```

</details>

#### Description


Test sending multiple blob transactions with the same nonce, but
higher gas tip so the transaction is replaced.


### Blob Transaction Ordering, Single Account, Single Blob (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transaction Ordering, Single Account, Single Blob (Cancun) (Client)"
```

</details>

#### Description


Send N blob transactions with `cancun.MAX_BLOBS_PER_BLOCK-1` blobs each,
using account A.

Using same account, and an increased nonce from the previously sent
transactions, send N blob transactions with 1 blob each.

Verify that the payloads are created with the correct ordering:
 - The first payloads must include the first N blob transactions
 - The last payloads must include the last single-blob transactions

All transactions have sufficient data gas price to be included any
of the payloads.


### Blob Transactions On Block 1, Shanghai Genesis (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transactions On Block 1, Shanghai Genesis (Cancun) (Client)"
```

</details>

#### Description


Tests the Cancun fork since Block 1.

Verifications performed:
- Correct implementation of Engine API changes for Cancun:
  - engine_newPayloadV3, engine_forkchoiceUpdatedV3, engine_getPayloadV3
- Correct implementation of EIP-4844:
  - Blob transaction ordering and inclusion
  - Blob transaction blob gas cost checks
  - Verify Blob bundle on built payload
- Eth RPC changes for Cancun:
  - Blob fields in eth_getBlockByNumber
  - Beacon root in eth_getBlockByNumber
  - Blob fields in transaction receipts from eth_getTransactionReceipt


### Blob Transaction Ordering, Single Account, Dual Blob (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transaction Ordering, Single Account, Dual Blob (Cancun) (Client)"
```

</details>

#### Description


Send N blob transactions with `cancun.MAX_BLOBS_PER_BLOCK-1` blobs each,
using account A.

Using same account, and an increased nonce from the previously sent
transactions, send a single 2-blob transaction, and send N blob
transactions with 1 blob each.
Verify that the payloads are created with the correct ordering:
 - The first payloads must include the first N blob transactions
 - The last payloads must include the rest of the transactions
All transactions have sufficient data gas price to be included any
of the payloads.


### Blob Transaction Ordering, Multiple Clients (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Blob Transaction Ordering, Multiple Clients (Cancun) (Client)"
```

</details>

#### Description


Send N blob transactions with `cancun.MAX_BLOBS_PER_BLOCK-1` blobs each,
using account A, to client A.
Send N blob transactions with 1 blob each from account B, to client
B.
Verify that the payloads are created with the correct ordering:
 - All payloads must have full blobs.
All transactions have sufficient data gas price to be included any
of the payloads.


### Parallel Blob Transactions (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Parallel Blob Transactions (Cancun) (Client)"
```

</details>

#### Description


Test sending multiple blob transactions in parallel from different accounts.

Verify that a payload is created with the maximum number of blobs.


## Category: Cancun: Fork Timestamp

### ForkchoiceUpdatedV2 then ForkchoiceUpdatedV3 Valid Payload Building Requests (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV2 then ForkchoiceUpdatedV3 Valid Payload Building Requests (Cancun) (Client)"
```

</details>

#### Description


Test requesting a Shanghai ForkchoiceUpdatedV2 payload followed by a Cancun ForkchoiceUpdatedV3 request.
Verify that client correctly returns the Cancun payload.


## Category: Cancun: GetPayloadV3

### GetPayloadV3 To Request Shanghai Payload (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/GetPayloadV3 To Request Shanghai Payload (Cancun) (Client)"
```

</details>

#### Description


Test requesting a Shanghai PayloadID using GetPayloadV3.
Verify that client returns UNSUPPORTED_FORK_ERROR.


### GetPayloadV2 To Request Cancun Payload (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/GetPayloadV2 To Request Cancun Payload (Cancun) (Client)"
```

</details>

#### Description


Test requesting a Cancun PayloadID using GetPayloadV2.
Verify that client returns UNSUPPORTED_FORK_ERROR.


## Category: Cancun: BlobGasUsed

### Incorrect BlobGasUsed, Non-Zero on Zero Blobs (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlobGasUsed, Non-Zero on Zero Blobs (Cancun) (Client)"
```

</details>

#### Description


Send a payload with zero blobs, but non-zero BlobGasUsed.


### Incorrect BlobGasUsed, GAS_PER_BLOB on Zero Blobs (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Incorrect BlobGasUsed, GAS_PER_BLOB on Zero Blobs (Cancun) (Client)"
```

</details>

#### Description


Send a payload with zero blobs, but non-zero BlobGasUsed.


## Category: Cancun: Blob Transactions DevP2P

### Request Blob Pooled Transactions (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/Request Blob Pooled Transactions (Cancun) (Client)"
```

</details>

#### Description


Requests blob pooled transactions and verify correct encoding.


## Category: Cancun: ForkchoiceUpdatedV3

### ForkchoiceUpdatedV3 Set Head to Shanghai Payload, Null Payload Attributes (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV3 Set Head to Shanghai Payload, Null Payload Attributes (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV3 to set the head of the chain to a Shanghai payload:
- Send NewPayloadV2 with Shanghai payload on block 1
- Use ForkchoiceUpdatedV3 to set the head to the payload, with null payload attributes

Verify that client returns no error


### ForkchoiceUpdatedV2 To Request Cancun Payload, Non-Null Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV2 To Request Cancun Payload, Non-Null Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
- Payload Attributes uses Cancun timestamp
- Payload Attributes Beacon Root is non-null

Verify that client returns INVALID_PARAMS_ERROR.


### ForkchoiceUpdatedV3 To Request Shanghai Payload, Null Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV3 To Request Shanghai Payload, Null Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
- Payload Attributes uses Shanghai timestamp
- Payload Attributes Beacon Root is null

Verify that client returns INVALID_PARAMS_ERROR.


### ForkchoiceUpdatedV3 To Request Shanghai Payload, Non-Null Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV3 To Request Shanghai Payload, Non-Null Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
- Payload Attributes uses Shanghai timestamp
- Payload Attributes Beacon Root is non-null

Verify that client returns UNSUPPORTED_FORK_ERROR.


### ForkchoiceUpdatedV3 Modifies Payload ID on Different Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV3 Modifies Payload ID on Different Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test requesting a Cancun Payload using ForkchoiceUpdatedV3 twice with the beacon root
payload attribute as the only change between requests and verify that the payload ID is
different.


### ForkchoiceUpdatedV2 To Request Shanghai Payload, Non-Null Beacon Root  (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV2 To Request Shanghai Payload, Non-Null Beacon Root  (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV2 to request a Shanghai payload:
- Payload Attributes uses Shanghai timestamp
- Payload Attributes Beacon Root is non-null

Verify that client returns INVALID_PARAMS_ERROR.


### ForkchoiceUpdatedV2 To Request Cancun Payload, Missing Beacon Root (Cancun) (Client)

#### Run

<details>
<summary>Command-line</summary>

```bash
./hive --client <CLIENTS> --sim ethereum/engine --sim.limit "engine-cancun/ForkchoiceUpdatedV2 To Request Cancun Payload, Missing Beacon Root (Cancun) (Client)"
```

</details>

#### Description


Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
- Payload Attributes uses Cancun timestamp
- Payload Attributes Beacon Root is missing

Verify that client returns UNSUPPORTED_FORK_ERROR.


