// # Test suite for cancun tests
package suite_cancun

import (
	"math/big"

	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
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
				BlobTransactionMaxBlobGasCost: big.NewInt(1000000000),
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
				BlobTransactionMaxBlobGasCost: big.NewInt(1000000000),
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
				BlobTransactionMaxBlobGasCost: big.NewInt(1000000000),
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
				BlobTransactionMaxBlobGasCost: big.NewInt(1000000000),
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
			Name: "Blob Transaction Ordering, Single Account, Dual Blob",
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
			NewPayloads{},

			// Send 5 single-blob transactions step-by-step
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			// Then send the dual-blob transaction, produce a block with 2 blobs.
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           2,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
			},
			NewPayloads{
				ExpectedIncludedBlobCount: cancun.MAX_BLOBS_PER_BLOCK,
			},

			// Final empty payload to advance
			NewPayloads{ExpectedIncludedBlobCount: 0},
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
			NewPayloads{},

			// Send 5 single-blob transactions from account A, validating one payload per tx
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			// Then send 5 single-blob transactions from account B, also validating per payload
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
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
				//SkipAddingToCLMock: true,
			},

			// Create a block without any blobs to get past genesis
			NewPayloads{
				PayloadCount:              1,
				ExpectedIncludedBlobCount: 0,
			},

			// First, send 5 single-blob transactions from account A to client A,
			// validating one payload per transaction
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           cancun.MAX_BLOBS_PER_BLOCK - 1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  0,
				ClientIndex:                   0,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			// Then, send 5 single-blob transactions from account B to client B,
			// validating one payload per transaction
			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},

			SendBlobTransactions{
				TransactionCount:              1,
				BlobsPerTransaction:           1,
				BlobTransactionGasTipCap:      big.NewInt(1e9),
				BlobTransactionMaxBlobGasCost: big.NewInt(200000000000),
				AccountIndex:                  1,
				ClientIndex:                   1,
			},
			NewPayloads{
				ExpectedIncludedBlobCount: 1,
				GetPayloadDelay:           2,
			},
		},
	},
}
