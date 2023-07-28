// # Test suite for blob tests
package suite_blobs

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	DATAHASH_START_ADDRESS = big.NewInt(0x100)
	DATAHASH_ADDRESS_COUNT = 1000

	// Fork specific constants
	GAS_PER_BLOB = uint64(0x20000)

	MIN_DATA_GASPRICE         = uint64(1)
	MAX_BLOB_GAS_PER_BLOCK    = uint64(786432)
	TARGET_BLOB_GAS_PER_BLOCK = uint64(393216)

	TARGET_BLOBS_PER_BLOCK = uint64(TARGET_BLOB_GAS_PER_BLOCK / GAS_PER_BLOB)
	MAX_BLOBS_PER_BLOCK    = uint64(MAX_BLOB_GAS_PER_BLOCK / GAS_PER_BLOB)

	BLOB_GASPRICE_UPDATE_FRACTION = uint64(3338477)

	BLOB_COMMITMENT_VERSION_KZG = byte(0x01)

	// Engine API errors
	INVALID_PARAMS_ERROR   = pInt(-32602)
	UNSUPPORTED_FORK_ERROR = pInt(-38005)
)

// Precalculate the first data gas cost increase
var (
	DATA_GAS_COST_INCREMENT_EXCEED_BLOBS = GetMinExcessBlobsForBlobGasPrice(2)
)

func pUint64(v uint64) *uint64 {
	return &v
}

func pInt(v int) *int {
	return &v
}

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

// List of all blob tests
var Tests = []test.SpecInterface{
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transactions On Block 1, Shanghai Genesis",
			About: `
			Tests the Cancun fork since Block 1.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 1,

		TestSequence: TestSequence{
			// We are starting at Shanghai genesis so send a couple payloads to reach the fork
			NewPayloads{},

			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				BlobTransactionSendCount:      DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(MAX_BLOBS_PER_BLOCK-TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (MAX_BLOBS_PER_BLOCK - TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transactions On Block 1, Cancun Genesis",
			About: `
			Tests the Cancun fork since genesis.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			NewPayloads{}, // Create a single empty payload to push the client through the fork.
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				BlobTransactionSendCount:      DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(MAX_BLOBS_PER_BLOCK-TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (MAX_BLOBS_PER_BLOCK - TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Single Account",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send N blob transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions.
			 - The last payloads must include the last single-blob transactions.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			// Then send the single-blob transactions
			SendBlobTransactions{
				BlobTransactionSendCount:      MAX_BLOBS_PER_BLOCK + 1,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First four payloads have MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              4,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              2,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Single Account 2",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send a single 2-blob transaction, and send N blob
			transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions.
			 - The last payloads must include the rest of the transactions.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the dual-blob transaction
			SendBlobTransactions{
				BlobTransactionSendCount:      1,
				BlobsPerTransaction:           2,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the single-blob transactions
			SendBlobTransactions{
				BlobTransactionSendCount:      MAX_BLOBS_PER_BLOCK - 2,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First five payloads have MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Accounts",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Send N blob transactions with 1 blob each from account B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  0,
			},
			// Then send the single-blob transactions from account B
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Blob Transaction Ordering, Multiple Clients",
			About: `
			Send N blob transactions with MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A, to client A.
			Send N blob transactions with 1 blob each from account B, to client
			B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Start a secondary client to also receive blob transactions
			LaunchClients{
				EngineStarter: hive_rpc.HiveRPCEngineStarter{},
				// Skip adding the second client to the CL Mock to guarantee
				// that all payloads are produced by client A.
				// This is done to not have client B prioritizing single-blob
				// transactions to fill one single payload.
				SkipAddingToCLMock: true,
			},

			// Create a block without any blobs to get past genesis
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: 0,
			},

			// First send the MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A, to client A.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(120),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			// Then send the single-blob transactions from account B, to client
			// B.
			SendBlobTransactions{
				BlobTransactionSendCount:      5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				// Wait a bit more on before requesting the built payload from the client
				GetPayloadDelay: 2,
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Replace Blob Transactions",
			About: `
			Test sending multiple blob transactions with the same nonce, but
			higher gas tip so the transaction is replaced.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			SendBlobTransactions{ // Blob ID 0
				BlobTransactionSendCount:      1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e9),
				BlobTransactionGasTipCap:      big.NewInt(1e9),
			},
			SendBlobTransactions{ // Blob ID 1
				BlobTransactionSendCount:      1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e2),
				BlobTransactionGasFeeCap:      big.NewInt(1e10),
				BlobTransactionGasTipCap:      big.NewInt(1e10),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 2
				BlobTransactionSendCount:      1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e3),
				BlobTransactionGasFeeCap:      big.NewInt(1e11),
				BlobTransactionGasTipCap:      big.NewInt(1e11),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 3
				BlobTransactionSendCount:      1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e4),
				BlobTransactionGasFeeCap:      big.NewInt(1e12),
				BlobTransactionGasTipCap:      big.NewInt(1e12),
				ReplaceTransactions:           true,
			},

			// We create the first payload, which must contain the blob tx
			// with the higher tip.
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				ExpectedBlobs:             []helper.BlobID{3},
			},
		},
	},

	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Parallel Blob Transactions",
			About: `
			Test sending multiple blob transactions in parallel from different accounts.
			`,
		},

		// We fork on genesis
		CancunForkHeight: 0,

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			ParallelSteps{
				Steps: []TestStep{
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  0,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  1,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  2,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  3,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  4,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  5,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  6,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  7,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  8,
					},
					SendBlobTransactions{
						BlobTransactionSendCount:      5,
						BlobsPerTransaction:           MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  9,
					},
				},
			},

			// We create the first payload, which is guaranteed to have the first MAX_BLOBS_PER_BLOCK blobs.
			NewPayloads{
				ExpectedIncludedBlobCount: MAX_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, MAX_BLOBS_PER_BLOCK),
			},
		},
	},

	// NewPayloadV3 Before Cancun, Negative Tests
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				VersionedHashes: &VersionedHashes{
					Blobs: nil,
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Nil Versioned Hashes",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- nil Versioned Hashes Array
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				PayloadCustomizer: &helper.CustomPayloadData{
					BlobGasUsed: pUint64(0),
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Nil Versioned Hashes",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				PayloadCustomizer: &helper.CustomPayloadData{
					ExcessBlobGas: pUint64(0),
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Empty Array Versioned Hashes",
			About: `
				Test sending NewPayloadV3 Before Cancun with:
				- nil ExcessBlobGas
				- nil BlobGasUsed
				- Empty Versioned Hashes Array
				`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{},
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Before Cancun, 0x00 Data Fields, Empty Array Versioned Hashes",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			`,
		},

		CancunForkHeight: 2,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{},
				},
				PayloadCustomizer: &helper.CustomPayloadData{
					ExcessBlobGas: pUint64(0),
					BlobGasUsed:   pUint64(0),
				},
				ExpectedError: UNSUPPORTED_FORK_ERROR,
			},
		},
	},

	// NewPayloadV3 After Cancun, Negative Tests
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 After Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Empty Array Versioned Hashes",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				PayloadCustomizer: &helper.CustomPayloadData{
					RemoveExcessBlobGas: true,
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 After Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Empty Array Versioned Hashes",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- Empty Versioned Hashes Array
			`,
		},

		CancunForkHeight: 1,

		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				Version:                   3,
				PayloadCustomizer: &helper.CustomPayloadData{
					RemoveBlobGasUsed: true,
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is missing one of the hashes.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK+1),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobListByIndex(helper.BlobID(TARGET_BLOBS_PER_BLOCK-1), 0),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK), helper.BlobID(TARGET_BLOBS_PER_BLOCK-1)),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob hash that does not belong to any blob contained in the payload.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(TARGET_BLOBS_PER_BLOCK)),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs:        helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
					HashVersions: []byte{BLOB_COMMITMENT_VERSION_KZG, BLOB_COMMITMENT_VERSION_KZG + 1},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: nil,
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				ExpectedBlobs:             []helper.BlobID{},
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{0},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3 on syncing clients
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash (Syncing)",
			About: `
				Tests VersionedHashes in Engine API NewPayloadV3 where the array
				is missing one of the hashes.
				`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK+1),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},
			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: helper.GetBlobListByIndex(helper.BlobID(TARGET_BLOBS_PER_BLOCK-1), 0),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK), helper.BlobID(TARGET_BLOBS_PER_BLOCK-1)),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: append(helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(TARGET_BLOBS_PER_BLOCK)),
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs:        helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
					HashVersions: []byte{BLOB_COMMITMENT_VERSION_KZG, BLOB_COMMITMENT_VERSION_KZG + 1},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: nil,
				},
				ExpectedError: INVALID_PARAMS_ERROR,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				BlobTransactionSendCount:      TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				ExpectedBlobs:             []helper.BlobID{},
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				VersionedHashes: &VersionedHashes{
					Blobs: []helper.BlobID{0},
				},
				ExpectedStatus: test.Invalid,
			},
		},
	},

	// BlobGasUsed, ExcessBlobGas Negative Tests
	// Most cases are contained in https://github.com/ethereum/execution-spec-tests/tree/main/tests/eips/eip4844
	// and can be executed using `pyspec` simulator.
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Incorrect BlobGasUsed: Non-Zero on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				PayloadCustomizer: &helper.CustomPayloadData{
					BlobGasUsed: pUint64(1),
				},
			},
		},
	},
	&BlobsBaseSpec{

		Spec: test.Spec{
			Name: "Incorrect BlobGasUsed: GAS_PER_BLOB on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
		},
		TestSequence: TestSequence{
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
				PayloadCustomizer: &helper.CustomPayloadData{
					BlobGasUsed: pUint64(GAS_PER_BLOB),
				},
			},
		},
	},

	// DevP2P tests
	&BlobsBaseSpec{
		Spec: test.Spec{
			Name: "Request Blob Pooled Transactions",
			About: `
			Requests blob pooled transactions and verify correct coding.
			`,
		},
		TestSequence: TestSequence{
			// Get past the genesis
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},
			// Send multiple transactions with multiple blobs each
			SendBlobTransactions{
				BlobTransactionSendCount:      1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			DevP2PRequestPooledTransactionHash{
				ClientIndex:                 0,
				TransactionIndexes:          []uint64{0},
				WaitForNewPooledTransaction: true,
			},
		},
	},
}

// Blobs base spec
// This struct contains the base spec for all blob tests. It contains the
// timestamp increments per block, the withdrawals fork height, and the list of
// payloads to produce during the test.
type BlobsBaseSpec struct {
	test.Spec
	TimeIncrements   uint64 // Timestamp increments per block throughout the test
	GetPayloadDelay  uint64 // Delay between FcU and GetPayload calls
	CancunForkHeight uint64 // Withdrawals activation fork height
	TestSequence
}

// Base test case execution procedure for blobs tests.
func (bs *BlobsBaseSpec) Execute(t *test.Env) {

	t.CLMock.WaitForTTD()

	blobTestCtx := &BlobTestContext{
		Env:            t,
		TestBlobTxPool: new(TestBlobTxPool),
	}

	blobTestCtx.TestBlobTxPool.HashesByIndex = make(map[uint64]common.Hash)

	if bs.GetPayloadDelay != 0 {
		t.CLMock.PayloadProductionClientDelay = time.Duration(bs.GetPayloadDelay) * time.Second
	}

	for stepId, step := range bs.TestSequence {
		t.Logf("INFO: Executing step %d: %s", stepId+1, step.Description())
		if err := step.Execute(blobTestCtx); err != nil {
			t.Fatalf("FAIL: Error executing step %d: %v", stepId+1, err)
		}
	}

}
