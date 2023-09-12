// # Test suite for cancun tests
package suite_cancun

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	suite_engine "github.com/ethereum/hive/simulators/ethereum/engine/suites/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

// Precalculate the first data gas cost increase
var (
	DATA_GAS_COST_INCREMENT_EXCEED_BLOBS = GetMinExcessBlobsForBlobGasPrice(2)
)

func pUint64(v uint64) *uint64 {
	return &v
}

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md

// List of all blob tests
var Tests = []test.Spec{
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transactions On Block 1, Shanghai Genesis",
			About: `
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
			`,
			MainFork: config.Cancun,
			// We fork after genesis
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			// We are starting at Shanghai genesis so send a couple payloads to reach the fork
			NewPayloads{},

			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				TransactionCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(cancun.MAX_BLOBS_PER_BLOCK-cancun.TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (cancun.MAX_BLOBS_PER_BLOCK - cancun.TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transactions On Block 1, Cancun Genesis",
			About: `
			Tests the Cancun fork since genesis.

			Verifications performed:
			* See Blob Transactions On Block 1, Shanghai Genesis
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			NewPayloads{}, // Create a single empty payload to push the client through the fork.
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// We create the first payload, and verify that the blob transactions
			// are included in the payload.
			// We also verify that the blob transactions are included in the blobs bundle.
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			// Try to increase the data gas cost of the blob transactions
			// by maxing out the number of blobs for the next payloads.
			SendBlobTransactions{
				TransactionCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS/(cancun.MAX_BLOBS_PER_BLOCK-cancun.TARGET_BLOBS_PER_BLOCK) + 1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},

			// Next payloads will have max data blobs each
			NewPayloads{
				PayloadCount:              DATA_GAS_COST_INCREMENT_EXCEED_BLOBS / (cancun.MAX_BLOBS_PER_BLOCK - cancun.TARGET_BLOBS_PER_BLOCK),
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},

			// But there will be an empty payload, since the data gas cost increased
			// and the last blob transaction was not included.
			NewPayloads{
				ExpectedIncludedBlobCount: 0,
			},

			// But it will be included in the next payload
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transaction Ordering, Single Account",
			About: `
			Send N blob transactions with cancun.MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send N blob transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions
			 - The last payloads must include the last single-blob transactions
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			// First send the cancun.MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			// Then send the single-blob transactions
			SendBlobTransactions{
				TransactionCount:              cancun.MAX_BLOBS_PER_BLOCK + 1,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First four payloads have cancun.MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              4,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              2,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},
		},
	},
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transaction Ordering, Single Account 2",
			About: `
			Send N blob transactions with cancun.MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Using same account, and an increased nonce from the previously sent
			transactions, send a single 2-blob transaction, and send N blob
			transactions with 1 blob each.
			Verify that the payloads are created with the correct ordering:
			 - The first payloads must include the first N blob transactions
			 - The last payloads must include the rest of the transactions
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			// First send the cancun.MAX_BLOBS_PER_BLOCK-1 blob transactions.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the dual-blob transaction
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           2,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// Then send the single-blob transactions
			SendBlobTransactions{
				TransactionCount:              cancun.MAX_BLOBS_PER_BLOCK - 2,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},

			// First five payloads have cancun.MAX_BLOBS_PER_BLOCK-1 blobs each
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK - 1,
			},

			// The rest of the payloads have full blobs
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transaction Ordering, Multiple Accounts",
			About: `
			Send N blob transactions with cancun.MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A.
			Send N blob transactions with 1 blob each from account B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			// First send the cancun.MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  0,
			},
			// Then send the single-blob transactions from account B
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},
		},
	},

	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Blob Transaction Ordering, Multiple Clients",
			About: `
			Send N blob transactions with cancun.MAX_BLOBS_PER_BLOCK-1 blobs each,
			using account A, to client A.
			Send N blob transactions with 1 blob each from account B, to client
			B.
			Verify that the payloads are created with the correct ordering:
			 - All payloads must have full blobs.
			All transactions have sufficient data gas price to be included any
			of the payloads.
			`,
			MainFork: config.Cancun,
		},

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

			// First send the cancun.MAX_BLOBS_PER_BLOCK-1 blob transactions from
			// account A, to client A.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionMaxBlobGasCost: big.NewInt(120),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			// Then send the single-blob transactions from account B, to client
			// B.
			SendBlobTransactions{
				TransactionCount:              5,
				BlobsPerTransaction:           1,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},

			// All payloads have full blobs
			NewPayloads{
				PayloadCount:              5,
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
				// Wait a bit more on before requesting the built payload from the client
				GetPayloadDelay: 2,
			},
		},
	},

	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Replace Blob Transactions",
			About: `
			Test sending multiple blob transactions with the same nonce, but
			higher gas tip so the transaction is replaced.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			SendBlobTransactions{ // Blob ID 0
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
				BlobTransactionGasFeeCap:      big.NewInt(1e9),
				BlobTransactionGasTipCap:      big.NewInt(1e9),
			},
			SendBlobTransactions{ // Blob ID 1
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e2),
				BlobTransactionGasFeeCap:      big.NewInt(1e10),
				BlobTransactionGasTipCap:      big.NewInt(1e10),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 2
				TransactionCount:              1,
				BlobTransactionMaxBlobGasCost: big.NewInt(1e3),
				BlobTransactionGasFeeCap:      big.NewInt(1e11),
				BlobTransactionGasTipCap:      big.NewInt(1e11),
				ReplaceTransactions:           true,
			},
			SendBlobTransactions{ // Blob ID 3
				TransactionCount:              1,
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

	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Parallel Blob Transactions",
			About: `
			Test sending multiple blob transactions in parallel from different accounts.

			Verify that a payload is created with the maximum number of blobs.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			// Send multiple blob transactions with the same nonce.
			ParallelSteps{
				Steps: []TestStep{
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  0,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  1,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  2,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  3,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  4,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  5,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  6,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  7,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  8,
					},
					SendBlobTransactions{
						TransactionCount:              5,
						BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
						BlobTransactionMaxBlobGasCost: big.NewInt(100),
						AccountIndex:                  9,
					},
				},
			},

			// We create the first payload, which is guaranteed to have the first cancun.MAX_BLOBS_PER_BLOCK blobs.
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.MAX_BLOBS_PER_BLOCK),
			},
		},
	},

	// ForkchoiceUpdatedV3 before cancun
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV3 Set Head to Shanghai Payload, Nil Payload Attributes",
			About: `
			Test sending ForkchoiceUpdatedV3 to set the head of the chain to a Shanghai payload:
			- Send NewPayloadV2 with Shanghai payload on block 1
			- Use ForkchoiceUpdatedV3 to set the head to the payload, with nil payload attributes

			Verify that client returns no error.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnHeadSet: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{},
				},
				ExpectationDescription: `
				ForkchoiceUpdatedV3 before Cancun returns no error without payload attributes
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV3 To Request Shanghai Payload, Nil Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root is nil

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV3 To Request Shanghai Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV3 to request a Shanghai payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							BeaconRoot: &(common.Hash{}),
						},
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV3 before Cancun with beacon root must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *globals.UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV2 before cancun with beacon root
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV2 To Request Shanghai Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Shanghai timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							BeaconRoot: &(common.Hash{}),
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 before Cancun with beacon root field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV2 after cancun
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV2 To Request Cancun Payload, Zero Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Cancun timestamp
			- Payload Attributes' Beacon Root zero

			Verify that client returns INVALID_PARAMS_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 after Cancun with beacon root field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV2 To Request Cancun Payload, Nil Beacon Root",
			About: `
			Test sending ForkchoiceUpdatedV2 to request a Cancun payload:
			- Payload Attributes uses Cancun timestamp
			- Payload Attributes' Beacon Root nil (not provided)

			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				FcUOnPayloadRequest: &helper.DowngradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							RemoveBeaconRoot: true,
						},
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				ForkchoiceUpdatedV2 after Cancun must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *globals.UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// ForkchoiceUpdatedV3 with modified BeaconRoot Attribute
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV3 Modifies Payload ID on Different Beacon Root",
			About: `
			Test requesting a Cancun Payload using ForkchoiceUpdatedV3 twice with the beacon root
			payload attribute as the only change between requests and verify that the payload ID is
			different.
			`,
			MainFork: config.Cancun,
		},

		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
						BeaconRoot: &(common.Hash{}),
					},
				},
			},
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(100),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
						BeaconRoot: &(common.Hash{1}),
					},
				},
			},
		},
	},

	// GetPayloadV3 Before Cancun, Negative Tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "GetPayloadV3 To Request Shanghai Payload",
			About: `
			Test requesting a Shanghai PayloadID using GetPayloadV3.
			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				GetPayloadCustomizer: &helper.UpgradeGetPayloadVersion{
					GetPayloadCustomizer: &helper.BaseGetPayloadCustomizer{
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				GetPayloadV3 To Request Shanghai Payload must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *globals.UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// GetPayloadV2 After Cancun, Negative Tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "GetPayloadV2 To Request Cancun Payload",
			About: `
			Test requesting a Cancun PayloadID using GetPayloadV2.
			Verify that client returns UNSUPPORTED_FORK_ERROR.
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				GetPayloadCustomizer: &helper.DowngradeGetPayloadVersion{
					GetPayloadCustomizer: &helper.BaseGetPayloadCustomizer{
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				GetPayloadV2 To Request Cancun Payload must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *globals.UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// NewPayloadV3 Before Cancun, Negative Tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root

			Verify that client returns INVALID_PARAMS_ERROR
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							VersionedHashesCustomizer: &VersionedHashes{
								Blobs: nil,
							},
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							BlobGasUsed: pUint64(0),
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Nil Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- nil Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ExcessBlobGas: pUint64(0),
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Empty Array Versioned Hashes, Nil Beacon Root",
			About: `
				Test sending NewPayloadV3 Before Cancun with:
				- nil ExcessBlobGas
				- nil BlobGasUsed
				- Empty Versioned Hashes Array
				- nil Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							VersionedHashesCustomizer: &VersionedHashes{
								Blobs: []helper.BlobID{},
							},
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, Nil Data Fields, Nil Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- nil ExcessBlobGas
			- nil BlobGasUsed
			- nil Versioned Hashes Array
			- Zero Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ParentBeaconRoot: &(common.Hash{}),
						},
						ExpectedError: globals.INVALID_PARAMS_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with any nil field must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Before Cancun, 0x00 Data Fields, Empty Array Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 Before Cancun with:
			- 0x00 ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			- Zero Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 2,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.UpgradeNewPayloadVersion{
					NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
						PayloadCustomizer: &helper.CustomPayloadData{
							ExcessBlobGas:    pUint64(0),
							BlobGasUsed:      pUint64(0),
							ParentBeaconRoot: &(common.Hash{}),
							VersionedHashesCustomizer: &VersionedHashes{
								Blobs: []helper.BlobID{},
							},
						},
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 before Cancun with no nil fields must return UNSUPPORTED_FORK_ERROR (code %d)
				`, *globals.UNSUPPORTED_FORK_ERROR),
			},
		},
	},

	// NewPayloadV3 After Cancun, Negative Tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 After Cancun, Nil ExcessBlobGas, 0x00 BlobGasUsed, Empty Array Versioned Hashes, Zero Beacon Root",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- nil ExcessBlobGas
			- 0x00 BlobGasUsed
			- Empty Versioned Hashes Array
			- Zero Beacon Root
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveExcessBlobGas: true,
					},
					ExpectedError: globals.INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil ExcessBlobGas must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 After Cancun, 0x00 ExcessBlobGas, Nil BlobGasUsed, Empty Array Versioned Hashes",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- Empty Versioned Hashes Array
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveBlobGasUsed: true,
					},
					ExpectedError: globals.INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil BlobGasUsed must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 After Cancun, 0x00 Blob Fields, Empty Array Versioned Hashes, Nil Beacon Root",
			About: `
			Test sending NewPayloadV3 After Cancun with:
			- 0x00 ExcessBlobGas
			- nil BlobGasUsed
			- Empty Versioned Hashes Array
			`,
			MainFork:   config.Cancun,
			ForkHeight: 1,
		},

		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						RemoveParentBeaconRoot: true,
					},
					ExpectedError: globals.INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: fmt.Sprintf(`
				NewPayloadV3 after Cancun with nil parentBeaconBlockRoot must return INVALID_PARAMS_ERROR (code %d)
				`, *globals.INVALID_PARAMS_ERROR),
			},
		},
	},

	// Fork time tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "ForkchoiceUpdatedV2 then ForkchoiceUpdatedV3 Valid Payload Building Requests",
			About: `
			Test requesting a Shanghai ForkchoiceUpdatedV2 payload followed by a Cancun ForkchoiceUpdatedV3 request.
			Verify that client correctly returns the Cancun payload.
			`,
			MainFork: config.Cancun,
			// We request two blocks from the client, first on shanghai and then on cancun, both with
			// the same parent.
			// Client must respond correctly to later request.
			ForkHeight:              1,
			BlockTimestampIncrement: 2,
		},

		TestSequence: TestSequence{
			// First, we send a couple of blob transactions on genesis,
			// with enough data gas cost to make sure they are included in the first block.
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				// This customizer only simulates requesting a Shanghai payload 1 second before cancun.
				// CL Mock will still request the Cancun payload afterwards
				FcUOnPayloadRequest: &helper.BaseForkchoiceUpdatedCustomizer{
					PayloadAttributesCustomizer: &helper.TimestampDeltaPayloadAttributesCustomizer{
						PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{
							RemoveBeaconRoot: true,
						},
						TimestampDelta: -1,
					},
				},
				ExpectationDescription: `
				ForkchoiceUpdatedV3 must construct transaction with blob payloads even if a ForkchoiceUpdatedV2 was previously requested
				`,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is missing one of the hashes.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK-1),
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
			MainFork: config.Cancun,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK+1),
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobListByIndex(helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK-1), 0),
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: append(helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK), helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK-1)),
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob hash that does not belong to any blob contained in the payload.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: append(helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK)),
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect hash in list of versioned hashes must return INVALID status
				`,
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs:        helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
							HashVersions: []byte{cancun.BLOB_COMMITMENT_VERSION_KZG, cancun.BLOB_COMMITMENT_VERSION_KZG + 1},
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect version in list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: nil,
						},
					},
					ExpectedError: globals.INVALID_PARAMS_ERROR,
				},
				ExpectationDescription: `
				NewPayloadV3 after Cancun with nil VersionedHashes must return INVALID_PARAMS_ERROR (code -32602)
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{},
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{
				ExpectedBlobs: []helper.BlobID{},
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{0},
						},
					},
					ExpectInvalidStatus: true,
				},
				ExpectationDescription: `
				NewPayloadV3 with incorrect list of versioned hashes must return INVALID status
				`,
			},
		},
	},

	// Test versioned hashes in Engine API NewPayloadV3 on syncing clients
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Missing Hash (Syncing)",
			About: `
				Tests VersionedHashes in Engine API NewPayloadV3 where the array
				is missing one of the hashes.
				`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK-1),
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Extra Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is has an extra hash for a blob that is not in the payload.
			`,
			MainFork: config.Cancun,
		},
		// TODO: It could be worth it to also test this with a blob that is in the
		// mempool but was not included in the payload.
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK+1),
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Out of Order (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is out of order.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},
			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: helper.GetBlobListByIndex(helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK-1), 0),
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Repeated Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: append(helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK), helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK-1)),
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Hash (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a blob that is repeated in the array.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: append(helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK-1), helper.BlobID(cancun.TARGET_BLOBS_PER_BLOCK)),
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Incorrect Version (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			has a single blob that has an incorrect version.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs:        helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
							HashVersions: []byte{cancun.BLOB_COMMITMENT_VERSION_KZG, cancun.BLOB_COMMITMENT_VERSION_KZG + 1},
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Nil Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is nil, even though the fork has already happened.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: nil,
						},
					},
					ExpectedError: globals.INVALID_PARAMS_ERROR,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is empty, even though there are blobs in the payload.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			SendBlobTransactions{
				TransactionCount:              cancun.TARGET_BLOBS_PER_BLOCK,
				BlobTransactionMaxBlobGasCost: big.NewInt(1),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.TARGET_BLOBS_PER_BLOCK,
				ExpectedBlobs:             helper.GetBlobList(0, cancun.TARGET_BLOBS_PER_BLOCK),
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{},
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "NewPayloadV3 Versioned Hashes, Non-Empty Hashes (Syncing)",
			About: `
			Tests VersionedHashes in Engine API NewPayloadV3 where the array
			is contains hashes, even though there are no blobs in the payload.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{}, // Send new payload so the parent is unknown to the secondary client
			NewPayloads{
				ExpectedBlobs: []helper.BlobID{},
			},

			LaunchClients{
				EngineStarter:            hive_rpc.HiveRPCEngineStarter{},
				SkipAddingToCLMock:       true,
				SkipConnectingToBootnode: true, // So the client is in a perpetual syncing state
			},
			SendModifiedLatestPayload{
				ClientID: 1,
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						VersionedHashesCustomizer: &VersionedHashes{
							Blobs: []helper.BlobID{0},
						},
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	// BlobGasUsed, ExcessBlobGas Negative Tests
	// Most cases are contained in https://github.com/ethereum/execution-spec-tests/tree/main/tests/cancun/eip4844_blobs
	// and can be executed using `pyspec` simulator.
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "Incorrect BlobGasUsed: Non-Zero on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						BlobGasUsed: pUint64(1),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},
	&CancunBaseSpec{

		BaseSpec: test.BaseSpec{
			Name: "Incorrect BlobGasUsed: GAS_PER_BLOB on Zero Blobs",
			About: `
			Send a payload with zero blobs, but non-zero BlobGasUsed.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			NewPayloads{
				NewPayloadCustomizer: &helper.BaseNewPayloadVersionCustomizer{
					PayloadCustomizer: &helper.CustomPayloadData{
						BlobGasUsed: pUint64(cancun.GAS_PER_BLOB),
					},
					ExpectInvalidStatus: true,
				},
			},
		},
	},

	// DevP2P tests
	&CancunBaseSpec{
		BaseSpec: test.BaseSpec{
			Name: "Request Blob Pooled Transactions",
			About: `
			Requests blob pooled transactions and verify correct encoding.
			`,
			MainFork: config.Cancun,
		},
		TestSequence: TestSequence{
			// Get past the genesis
			NewPayloads{
				PayloadCount: 1,
			},
			// Send multiple transactions with multiple blobs each
			SendBlobTransactions{
				TransactionCount:              1,
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

var EngineAPITests []test.Spec

func init() {
	// Append all engine api tests with Cancun as main fork
	for _, test := range suite_engine.Tests {
		Tests = append(Tests, test.WithMainFork(config.Cancun))
	}

	// Cancun specific variants for pre-existing tests
	baseSpec := test.BaseSpec{
		MainFork: config.Cancun,
	}
	onlyBlobTxsSpec := test.BaseSpec{
		MainFork:            config.Cancun,
		TestTransactionType: helper.BlobTxOnly,
	}

	// Payload Attributes
	for _, t := range []suite_engine.InvalidPayloadAttributesTest{
		{
			BaseSpec:    baseSpec,
			Description: "Missing BeaconRoot",
			Customizer: &helper.BasePayloadAttributesCustomizer{
				RemoveBeaconRoot: true,
			},
			// Error is expected on syncing because V3 checks all fields to be present
			ErrorOnSync: true,
		},
	} {
		Tests = append(Tests, t)
		t.Syncing = true
		Tests = append(Tests, t)
	}

	// Invalid Payload Tests
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidParentBeaconBlockRoot,
		helper.InvalidBlobGasUsed,
		helper.InvalidBlobCountGasUsed,
		helper.InvalidExcessBlobGas,
		helper.InvalidVersionedHashes,
		helper.InvalidVersionedHashesVersion,
		helper.IncompleteVersionedHashes,
		helper.ExtraVersionedHashes,
	} {
		for _, syncing := range []bool{false, true} {
			// Invalidity of payload can be detected even when syncing because the
			// blob gas only depends on the transactions contained.
			invalidDetectedOnSync := (invalidField == helper.InvalidBlobGasUsed ||
				invalidField == helper.InvalidBlobCountGasUsed ||
				invalidField == helper.InvalidVersionedHashes ||
				invalidField == helper.InvalidVersionedHashesVersion)

			Tests = append(Tests, suite_engine.InvalidPayloadTestCase{
				BaseSpec:              onlyBlobTxsSpec,
				InvalidField:          invalidField,
				Syncing:               syncing,
				InvalidDetectedOnSync: invalidDetectedOnSync,
			})
		}
	}

	// Invalid Transaction ChainID Tests
	Tests = append(Tests,
		suite_engine.InvalidTxChainIDTest{
			BaseSpec: onlyBlobTxsSpec,
		},
	)

	Tests = append(Tests, suite_engine.PayloadBuildAfterInvalidPayloadTest{
		BaseSpec:     onlyBlobTxsSpec,
		InvalidField: helper.InvalidParentBeaconBlockRoot,
	})

	// Suggested Fee Recipient Tests (New Transaction Type)
	Tests = append(Tests,
		suite_engine.SuggestedFeeRecipientTest{
			BaseSpec:         onlyBlobTxsSpec,
			TransactionCount: 1, // Only one blob tx gets through due to blob gas limit
		},
	)
	// Prev Randao Tests (New Transaction Type)
	Tests = append(Tests,
		suite_engine.PrevRandaoTransactionTest{
			BaseSpec: onlyBlobTxsSpec,
		},
	)
}
