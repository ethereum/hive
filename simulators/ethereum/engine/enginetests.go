package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/hivesim"
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

var engineTests = []TestSpec{

	// Engine API Negative Test Cases
	{
		Name: "Invalid Terminal Block in ForkchoiceUpdated",
		Run:  invalidTerminalBlockForkchoiceUpdated,
		TTD:  1000000,
	},
	{
		Name: "Invalid GetPayload Under PoW",
		Run:  invalidGetPayloadUnderPoW,
		TTD:  1000000,
	},
	{
		Name: "Invalid Terminal Block in NewPayload",
		Run:  invalidTerminalBlockNewPayload,
		TTD:  1000000,
	},
	{
		Name: "Unknown HeadBlockHash",
		Run:  unknownHeadBlockHash,
	},
	{
		Name: "Unknown SafeBlockHash",
		Run:  unknownSafeBlockHash,
	},
	{
		Name: "Unknown FinalizedBlockHash",
		Run:  unknownFinalizedBlockHash,
	},
	{
		Name: "ForkchoiceUpdated Invalid Payload Attributes",
		Run:  invalidPayloadAttributesGen(false),
	},
	{
		Name: "ForkchoiceUpdated Invalid Payload Attributes (Syncing)",
		Run:  invalidPayloadAttributesGen(true),
	},
	{
		Name: "Pre-TTD ForkchoiceUpdated After PoS Switch",
		Run:  preTTDFinalizedBlockHash,
		TTD:  2,
	},
	{
		Name: "Bad Hash on NewPayload",
		Run:  badHashOnNewPayloadGen(false, false),
	},
	{
		Name: "Bad Hash on NewPayload Syncing",
		Run:  badHashOnNewPayloadGen(true, false),
	},
	{
		Name: "Bad Hash on NewPayload Side Chain",
		Run:  badHashOnNewPayloadGen(false, true),
	},
	{
		Name: "Bad Hash on NewPayload Side Chain Syncing",
		Run:  badHashOnNewPayloadGen(true, true),
	},
	{
		Name: "ParentHash==BlockHash on NewPayload",
		Run:  parentHashOnExecPayload,
	},
	{
		Name: "Invalid ParentHash NewPayload",
		Run:  invalidPayloadTestCaseGen("ParentHash", false),
	},
	{
		Name: "Invalid ParentHash NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("ParentHash", true),
	},
	{
		Name: "Invalid StateRoot NewPayload",
		Run:  invalidPayloadTestCaseGen("StateRoot", false),
	},
	{
		Name: "Invalid StateRoot NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("StateRoot", true),
	},
	{
		Name: "Invalid ReceiptsRoot NewPayload",
		Run:  invalidPayloadTestCaseGen("ReceiptsRoot", false),
	},
	{
		Name: "Invalid ReceiptsRoot NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("ReceiptsRoot", true),
	},
	{
		Name: "Invalid Number NewPayload",
		Run:  invalidPayloadTestCaseGen("Number", false),
	},
	{
		Name: "Invalid Number NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Number", true),
	},
	{
		Name: "Invalid GasLimit NewPayload",
		Run:  invalidPayloadTestCaseGen("GasLimit", false),
	},
	{
		Name: "Invalid GasLimit NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("GasLimit", true),
	},
	{
		Name: "Invalid GasUsed NewPayload",
		Run:  invalidPayloadTestCaseGen("GasUsed", false),
	},
	{
		Name: "Invalid GasUsed NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("GasUsed", true),
	},
	{
		Name: "Invalid Timestamp NewPayload",
		Run:  invalidPayloadTestCaseGen("Timestamp", false),
	},
	{
		Name: "Invalid Timestamp NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Timestamp", true),
	},
	{
		Name: "Invalid PrevRandao NewPayload",
		Run:  invalidPayloadTestCaseGen("PrevRandao", false),
	},
	{
		Name: "Invalid PrevRandao NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("PrevRandao", true),
	},
	{
		Name: "Invalid Incomplete Transactions NewPayload",
		Run:  invalidPayloadTestCaseGen("RemoveTransaction", false),
	},
	{
		Name: "Invalid Incomplete Transactions NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("RemoveTransaction", true),
	},
	{
		Name: "Invalid Transaction Signature NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Signature", false),
	},
	{
		Name: "Invalid Transaction Signature NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Transaction/Signature", true),
	},
	{
		Name: "Invalid Transaction Nonce NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Nonce", false),
	},
	{
		Name: "Invalid Transaction Nonce NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Transaction/Nonce", true),
	},
	{
		Name: "Invalid Transaction GasPrice NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/GasPrice", false),
	},
	{
		Name: "Invalid Transaction GasPrice NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Transaction/GasPrice", true),
	},
	{
		Name: "Invalid Transaction Gas NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Gas", false),
	},
	{
		Name: "Invalid Transaction Gas NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Transaction/Gas", true),
	},
	{
		Name: "Invalid Transaction Value NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Value", false),
	},
	{
		Name: "Invalid Transaction Value NewPayload (Syncing)",
		Run:  invalidPayloadTestCaseGen("Transaction/Value", true),
	},

	// Eth RPC Status on ForkchoiceUpdated Events
	{
		Name: "Latest Block after NewPayload",
		Run:  blockStatusExecPayload,
	},
	{
		Name: "Latest Block after New HeadBlock",
		Run:  blockStatusHeadBlock,
	},
	{
		Name: "Latest Block after New SafeBlock",
		Run:  blockStatusSafeBlock,
	},
	{
		Name: "Latest Block after New FinalizedBlock",
		Run:  blockStatusFinalizedBlock,
	},
	{
		Name: "Latest Block after Reorg",
		Run:  blockStatusReorg,
	},

	// Payload Tests
	{
		Name: "Re-Execute Payload",
		Run:  reExecPayloads,
	},
	{
		Name: "Multiple New Payloads Extending Canonical Chain",
		Run:  multipleNewCanonicalPayloads,
	},
	{
		Name: "Out of Order Payload Execution",
		Run:  outOfOrderPayloads,
	},
	{
		Name: "Valid NewPayload->ForkchoiceUpdated on Syncing Client",
		Run:  validPayloadFcUSyncingClient,
	},

	// Transaction Reorg using Engine API
	{
		Name: "Transaction Reorg",
		Run:  transactionReorg,
	},
	{
		Name: "Sidechain Reorg",
		Run:  sidechainReorg,
	},
	{
		Name: "Re-Org Back into Canonical Chain",
		Run:  reorgBack,
	},

	// Suggested Fee Recipient in Payload creation
	{
		Name: "Suggested Fee Recipient Test",
		Run:  suggestedFeeRecipient,
	},

	// PrevRandao opcode tests
	{
		Name: "PrevRandao Opcode Transactions",
		Run:  prevRandaoOpcodeTx,
		TTD:  10,
	},

	// Multi-Client Sync tests
	{
		Name: "Sync Client Post Merge",
		Run:  postMergeSync,
		TTD:  10,
	},
}

// Invalid Terminal Block in ForkchoiceUpdated: Client must reject ForkchoiceUpdated directives if the referenced HeadBlockHash does not meet the TTD requirement.
func invalidTerminalBlockForkchoiceUpdated(t *TestEnv) {
	gblock := loadGenesisBlock(t.ClientFiles["/genesis.json"])

	forkchoiceState := ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}

	// Execution specification:
	// {payloadStatus: {status: INVALID_TERMINAL_BLOCK, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
	// either obtained from the Payload validation process or as a result of validating a PoW block referenced by forkchoiceState.headBlockHash
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceState, nil)
	r.ExpectPayloadStatus(InvalidTerminalBlock)
	r.ExpectLatestValidHash(nil)
	// ValidationError is not validated since it can be either null or a string message
}

// Invalid GetPayload Under PoW: Client must reject GetPayload directives under PoW.
func invalidGetPayloadUnderPoW(t *TestEnv) {
	// We start in PoW and try to get an invalid Payload, which should produce an error but nothing should be disrupted.
	r := t.TestEngine.TestEngineGetPayloadV1(&PayloadID{1, 2, 3, 4, 5, 6, 7, 8})
	r.ExpectError()

}

// Invalid Terminal Block in NewPayload: Client must reject NewPayload directives if the referenced ParentHash does not meet the TTD requirement.
func invalidTerminalBlockNewPayload(t *TestEnv) {
	gblock := loadGenesisBlock(t.ClientFiles["/genesis.json"])

	// Create a dummy payload to send in the NewPayload call
	payload := ExecutableDataV1{
		ParentHash:    gblock.Hash(),
		FeeRecipient:  common.Address{},
		StateRoot:     gblock.Root(),
		ReceiptsRoot:  types.EmptyUncleHash,
		LogsBloom:     types.CreateBloom(types.Receipts{}).Bytes(),
		PrevRandao:    common.Hash{},
		Number:        1,
		GasLimit:      gblock.GasLimit(),
		GasUsed:       0,
		Timestamp:     gblock.Time() + 1,
		ExtraData:     []byte{},
		BaseFeePerGas: gblock.BaseFee(),
		BlockHash:     common.Hash{},
		Transactions:  [][]byte{},
	}
	hashedPayload, err := customizePayload(&payload, &CustomPayloadData{})
	if err != nil {
		t.Fatalf("FAIL (%s): Error while constructing PoW payload: %v", t.TestName, err)
	}

	// Execution specification:
	// {status: INVALID_TERMINAL_BLOCK, latestValidHash: null, validationError: errorMessage | null}
	// if terminal block conditions are not satisfied
	r := t.TestEngine.TestEngineNewPayloadV1(hashedPayload)
	r.ExpectStatus(InvalidTerminalBlock)
	r.ExpectLatestValidHash(nil)
	// ValidationError is not validated since it can be either null or a string message
}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using NewPayload) and unknown SafeBlock
// results in error
func unknownSafeBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnNewPayloadBroadcast: func() {

			// Generate a random SafeBlock hash
			randomSafeBlockHash := common.Hash{}
			rand.Read(randomSafeBlockHash[:])

			// Send forkchoiceUpdated with random SafeBlockHash
			// Execution specification:
			// - This value MUST be either equal to or an ancestor of headBlockHash
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(
				&ForkchoiceStateV1{
					HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
					SafeBlockHash:      randomSafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil)
			r.ExpectError()

		},
	})

}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using NewPayload) and unknown
// FinalizedBlockHash results in error
func unknownFinalizedBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a new payload has been broadcast
		OnNewPayloadBroadcast: func() {

			// Generate a random FinalizedBlockHash hash
			randomFinalizedBlockHash := common.Hash{}
			rand.Read(randomFinalizedBlockHash[:])

			// Send forkchoiceUpdated with random FinalizedBlockHash
			forkchoiceStateUnknownFinalizedHash := ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: randomFinalizedBlockHash,
			}
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownFinalizedHash, nil)
			r.ExpectError()

			// Test again using PayloadAttributes, should also return INVALID and no PayloadID
			r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownFinalizedHash,
				&PayloadAttributesV1{
					Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
					PrevRandao:            common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				})
			r.ExpectError()

		},
	})

}

// Verify that an unknown hash at HeadBlock in the forkchoice results in client returning Syncing state
func unknownHeadBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// Generate a random HeadBlock hash
	randomHeadBlockHash := common.Hash{}
	rand.Read(randomHeadBlockHash[:])

	forkchoiceStateUnknownHeadHash := ForkchoiceStateV1{
		HeadBlockHash:      randomHeadBlockHash,
		SafeBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
		FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
	}

	t.Logf("INFO (%v) forkchoiceStateUnknownHeadHash: %v\n", t.TestName, forkchoiceStateUnknownHeadHash)

	// Execution specification::
	// - {payloadStatus: {status: SYNCING, latestValidHash: null, validationError: null}, payloadId: null}
	//   if forkchoiceState.headBlockHash references an unknown payload or a payload that can't be validated
	//   because requisite data for the validation is missing
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownHeadHash, nil)
	r.ExpectPayloadStatus(Syncing)

	// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
	r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceStateUnknownHeadHash,
		&PayloadAttributesV1{
			Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
			PrevRandao:            common.Hash{},
			SuggestedFeeRecipient: common.Address{},
		})
	r.ExpectPayloadStatus(Syncing)
	r.ExpectPayloadID(nil)

}

// Verify behavior on a forkchoiceUpdated with invalid payload attributes
func invalidPayloadAttributesGen(syncing bool) func(*TestEnv) {

	return func(t *TestEnv) {
		// Wait until TTD is reached by this client
		t.CLMock.waitForTTD()

		// Produce blocks before starting the test
		t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

		// Send a forkchoiceUpdated with invalid PayloadAttributes
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			OnNewPayloadBroadcast: func() {
				// Try to apply the new payload with invalid attributes
				var blockHash common.Hash
				if syncing {
					// Setting a random hash will put the client into `SYNCING`
					rand.Read(blockHash[:])
				} else {
					// Set the block hash to the next payload that was broadcasted
					blockHash = t.CLMock.LatestPayloadBuilt.BlockHash
				}
				t.Logf("INFO (%s): Sending EngineForkchoiceUpdatedV1 (Syncing=%s) with invalid payload attributes", t.TestName, syncing)
				fcu := ForkchoiceStateV1{
					HeadBlockHash:      blockHash,
					SafeBlockHash:      blockHash,
					FinalizedBlockHash: blockHash,
				}
				attr := PayloadAttributesV1{
					Timestamp:             0,
					PrevRandao:            common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				}
				// 0) Check headBlock is known and there is no missing data, if not respond with SYNCING
				// 1) Check headBlock is VALID, if not respond with INVALID
				// 2) Apply forkchoiceState
				// 3) Check payloadAttributes, if invalid respond with error: code: Invalid payload attributes
				// 4) Start payload build process and respond with VALID
				if syncing {
					// If we are SYNCING, the outcome should be SYNCING regardless of the validity of the payload atttributes
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcu, &attr)
					r.ExpectPayloadStatus(Syncing)
					r.ExpectPayloadID(nil)
				} else {
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcu, &attr)
					r.ExpectError()

					// Check that the forkchoice was applied, regardless of the error
					s := t.TestEth.TestHeaderByNumber(nil)
					s.ExpectHash(blockHash)
				}
			},
		})
	}

}

// Verify that a forkchoiceUpdated fails on hash being set to a pre-TTD block after PoS change
func preTTDFinalizedBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// Send the Genesis block as forkchoice
	gblock := loadGenesisBlock(t.ClientFiles["/genesis.json"])

	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}, nil)
	r.ExpectPayloadStatus(InvalidTerminalBlock)

	r = t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
	r.ExpectPayloadStatus(Valid)

}

// Corrupt the hash of a valid payload, client should reject the payload.
// All possible scenarios:
//    (fcU)
//	┌────────┐        ┌────────────────────────┐
//	│  HEAD  │◄───────┤ Bad Hash (!Sync,!Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//		 │
//	┌────▼───┐        ┌────────────────────────┐
//	│ HEAD-1 │◄───────┤ Bad Hash (!Sync, Side) │
//	└────┬───┘        └────────────────────────┘
//		 │
//
//
//	  (fcU)
//	********************  ┌───────────────────────┐
//	*  (Unknown) HEAD  *◄─┤ Bad Hash (Sync,!Side) │
//	********************  └───────────────────────┘
//		 │
//		 │
//	┌────▼───┐            ┌───────────────────────┐
//	│ HEAD-1 │◄───────────┤ Bad Hash (Sync, Side) │
//	└────┬───┘            └───────────────────────┘
//		 │
//

func badHashOnNewPayloadGen(syncing bool, sidechain bool) func(*TestEnv) {

	return func(t *TestEnv) {
		// Wait until TTD is reached by this client
		t.CLMock.waitForTTD()

		// Produce blocks before starting the test
		t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

		var (
			alteredPayload     ExecutableDataV1
			invalidPayloadHash common.Hash
		)

		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				// Alter hash on the payload and send it to client, should produce an error
				alteredPayload = t.CLMock.LatestPayloadBuilt
				invalidPayloadHash = alteredPayload.BlockHash
				invalidPayloadHash[common.HashLength-1] = byte(255 - invalidPayloadHash[common.HashLength-1])
				alteredPayload.BlockHash = invalidPayloadHash

				if !syncing && sidechain {
					// We alter the payload by setting the parent to a known past block in the
					// canonical chain, which makes this payload a side chain payload, and also an invalid block hash
					// (because we did not update the block hash appropriately)
					alteredPayload.ParentHash = t.CLMock.LatestFinalizedHeader.ParentHash
				} else if syncing {
					// We need to send an fcU to put the client in SYNCING state.
					randomHeadBlock := common.Hash{}
					rand.Read(randomHeadBlock[:])
					fcU := ForkchoiceStateV1{
						HeadBlockHash:      randomHeadBlock,
						SafeBlockHash:      t.CLMock.LatestFinalizedHeader.Hash(),
						FinalizedBlockHash: t.CLMock.LatestFinalizedHeader.Hash(),
					}
					r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcU, nil)
					r.ExpectPayloadStatus(Syncing)

					if sidechain {
						// Syncing and sidechain, the caonincal head is an unknown payload to us,
						// but this specific bad hash payload is in theory part of a side chain.
						// Therefore the parent we use is the head hash.
						alteredPayload.ParentHash = t.CLMock.LatestFinalizedHeader.Hash()
					} else {
						// The invalid bad-hash payload points to the unknown head, but we know it is
						// indeed canonical because the head was set using forkchoiceUpdated.
						alteredPayload.ParentHash = randomHeadBlock
					}
				}

				// Execution specification::
				// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
				r := t.TestEngine.TestEngineNewPayloadV1(&alteredPayload)
				r.ExpectStatus(InvalidBlockHash)
				r.ExpectLatestValidHash(nil)
			},
		})

		// Lastly, attempt to build on top of the invalid payload
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				alteredPayload, err := customizePayload(&t.CLMock.LatestPayloadBuilt, &CustomPayloadData{
					ParentHash: &invalidPayloadHash,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
				}

				// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
				// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
				// but a VALID response is incorrect.
				r := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
				r.ExpectStatusEither(Accepted, Invalid, Syncing)

			},
		})

	}
}

// Copy the parentHash into the blockHash, client should reject the payload
// (from Kintsugi Incident Report: https://notes.ethereum.org/@ExXcnR0-SJGthjz1dwkA1A/BkkdHWXTY)
func parentHashOnExecPayload(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload := t.CLMock.LatestPayloadBuilt
			alteredPayload.BlockHash = alteredPayload.ParentHash
			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			r := t.TestEngine.TestEngineNewPayloadV1(&alteredPayload)
			r.ExpectStatus(InvalidBlockHash)
		},
	})

}

// Generate test cases for each field of NewPayload, where the payload contains a single invalid field and a valid hash.
func invalidPayloadTestCaseGen(payloadField string, syncing bool) func(*TestEnv) {
	return func(t *TestEnv) {

		if syncing {
			// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
			secondaryClient, _, err := t.StartClient(t.Client.Type, t.ClientParams, t.MainTTD())
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
			}
			t.CLMock.AddEngineClient(t.T, secondaryClient, t.MainTTD())
		}

		// Wait until TTD is reached by all clients
		t.CLMock.waitForTTD()

		txFunc := func() {
			// Function to send at least one transaction each block produced
			// Send the transaction to the prevRandaoContractAddr
			t.sendNextTransaction(t.CLMock.NextBlockProducer, prevRandaoContractAddr, big1, nil)
		}

		// Produce blocks before starting the test
		t.CLMock.produceBlocks(5, BlockProcessCallbacks{
			// Make sure at least one transaction is included in each block
			OnPayloadProducerSelected: txFunc,
		})

		if syncing {
			// Disconnect the main engine client from the CL Mocker and produce a block
			t.CLMock.RemoveEngineClient(t.Client)
			t.CLMock.produceSingleBlock(BlockProcessCallbacks{
				OnPayloadProducerSelected: txFunc,
			})

			// This block is now unknown to the main client, sending an fcU will set it to syncing mode
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
			r.ExpectPayloadStatus(Syncing)
		}

		var invalidPayloadHash common.Hash

		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			// Make sure at least one transaction is included in the payload
			OnPayloadProducerSelected: txFunc,
			// Run test after the new payload has been obtained
			OnGetPayload: func() {

				// Alter the payload while maintaining a valid hash and send it to the client, should produce an error
				basePayload := t.CLMock.LatestPayloadBuilt
				var customPayloadMod *CustomPayloadData
				payloadFieldSplit := strings.Split(payloadField, "/")
				switch payloadFieldSplit[0] {
				case "ParentHash":
					modParentHash := basePayload.ParentHash
					modParentHash[common.HashLength-1] = byte(255 - modParentHash[common.HashLength-1])
					customPayloadMod = &CustomPayloadData{
						ParentHash: &modParentHash,
					}
				case "StateRoot":
					modStateRoot := basePayload.StateRoot
					modStateRoot[common.HashLength-1] = byte(255 - modStateRoot[common.HashLength-1])
					customPayloadMod = &CustomPayloadData{
						StateRoot: &modStateRoot,
					}
				case "ReceiptsRoot":
					modReceiptsRoot := basePayload.ReceiptsRoot
					modReceiptsRoot[common.HashLength-1] = byte(255 - modReceiptsRoot[common.HashLength-1])
					customPayloadMod = &CustomPayloadData{
						ReceiptsRoot: &modReceiptsRoot,
					}
				case "Number":
					modNumber := basePayload.Number - 1
					customPayloadMod = &CustomPayloadData{
						Number: &modNumber,
					}
				case "GasLimit":
					modGasLimit := basePayload.GasLimit * 2
					customPayloadMod = &CustomPayloadData{
						GasLimit: &modGasLimit,
					}
				case "GasUsed":
					modGasUsed := basePayload.GasUsed - 1
					customPayloadMod = &CustomPayloadData{
						GasUsed: &modGasUsed,
					}
				case "Timestamp":
					modTimestamp := basePayload.Timestamp - 1
					customPayloadMod = &CustomPayloadData{
						Timestamp: &modTimestamp,
					}
				case "PrevRandao":
					// This should fail since we are inserting a transaction that uses the PREVRANDAO opcode.
					// The expected outcome will change if we modify the payload.
					if len(basePayload.Transactions) == 0 {
						// But if the payload has no transactions, the test is invalid
						t.Fatalf("FAIL (%s): No transactions in the base payload", t.TestName)
					}
					modPrevRandao := common.Hash{}
					rand.Read(modPrevRandao[:])
					customPayloadMod = &CustomPayloadData{
						PrevRandao: &modPrevRandao,
					}
				case "RemoveTransaction":
					emptyTxs := make([][]byte, 0)
					customPayloadMod = &CustomPayloadData{
						Transactions: &emptyTxs,
					}
				case "Transaction":
					if len(payloadFieldSplit) < 2 {
						t.Fatalf("FAIL (%s): No transaction field specified: %s", t.TestName, payloadField)
					}
					if len(basePayload.Transactions) == 0 {
						t.Fatalf("FAIL (%s): No transactions available for modification", t.TestName)
					}
					var baseTx types.Transaction
					if err := baseTx.UnmarshalBinary(basePayload.Transactions[0]); err != nil {
						t.Fatalf("FAIL (%s): Unable to unmarshal binary tx: %v", t.TestName, err)
					}
					var customTxData CustomTransactionData
					switch payloadFieldSplit[1] {
					case "Signature":
						modifiedSignature := SignatureValuesFromRaw(baseTx.RawSignatureValues())
						modifiedSignature.R = modifiedSignature.R.Sub(modifiedSignature.R, big1)
						customTxData = CustomTransactionData{
							Signature: &modifiedSignature,
						}
					case "Nonce":
						customNonce := baseTx.Nonce() - 1
						customTxData = CustomTransactionData{
							Nonce: &customNonce,
						}
					case "Gas":
						customGas := uint64(0)
						customTxData = CustomTransactionData{
							Gas: &customGas,
						}
					case "GasPrice":
						customTxData = CustomTransactionData{
							GasPrice: big0,
						}
					case "Value":
						// Vault account initially has 0x123450000000000000000, so this value should overflow
						customValue, err := hexutil.DecodeBig("0x123450000000000000001")
						if err != nil {
							t.Fatalf("FAIL (%s): Unable to prepare custom tx value: %v", t.TestName, err)
						}
						customTxData = CustomTransactionData{
							Value: customValue,
						}
					}

					modifiedTx, err := customizeTransaction(&baseTx, vaultKey, &customTxData)
					if err != nil {
						t.Fatalf("FAIL (%s): Unable to modify tx: %v", t.TestName, err)
					}
					t.Logf("INFO (%s): Modified tx %v / original tx %v", t.TestName, baseTx, modifiedTx)

					modifiedTxBytes, err := modifiedTx.MarshalBinary()
					if err != nil {
					}
					modifiedTransactions := [][]byte{
						modifiedTxBytes,
					}
					customPayloadMod = &CustomPayloadData{
						Transactions: &modifiedTransactions,
					}
				}

				if customPayloadMod == nil {
					t.Fatalf("FAIL (%s): Invalid test case: %s", t.TestName, payloadField)
				}

				t.Logf("INFO (%v) customizing payload using: %s\n", t.TestName, customPayloadMod.String())
				alteredPayload, err := customizePayload(&t.CLMock.LatestPayloadBuilt, customPayloadMod)
				t.Logf("INFO (%v) latest real getPayload (not executed): hash=%v contents=%v\n", t.TestName, t.CLMock.LatestPayloadBuilt.BlockHash, t.CLMock.LatestPayloadBuilt)
				t.Logf("INFO (%v) customized payload: hash=%v contents=%v\n", t.TestName, alteredPayload.BlockHash, alteredPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload (%v): %v", t.TestName, customPayloadMod, err)
				}
				invalidPayloadHash = alteredPayload.BlockHash

				// Depending on the field we modified, we expect a different status
				r := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
				if syncing || payloadField == "ParentHash" {
					// Execution specification::
					// {status: ACCEPTED, latestValidHash: null, validationError: null} if the following conditions are met:
					//  - the blockHash of the payload is valid
					//  - the payload doesn't extend the canonical chain
					//  - the payload hasn't been fully validated
					// {status: SYNCING, latestValidHash: null, validationError: null}
					// if the payload extends the canonical chain and requisite data for its validation is missing
					// (the client can assume the payload extends the canonical because the linking payload could be missing)
					r.ExpectStatusEither(Accepted, Syncing)
					r.ExpectLatestValidHash(nil)
				} else {
					r.ExpectStatus(Invalid)
					r.ExpectLatestValidHash(&alteredPayload.ParentHash)
				}

				// Send the forkchoiceUpdated with a reference to the invalid payload.
				fcState := ForkchoiceStateV1{
					HeadBlockHash:      alteredPayload.BlockHash,
					SafeBlockHash:      alteredPayload.BlockHash,
					FinalizedBlockHash: alteredPayload.BlockHash,
				}
				payloadAttrbutes := PayloadAttributesV1{
					Timestamp:             alteredPayload.Timestamp + 1,
					PrevRandao:            common.Hash{},
					SuggestedFeeRecipient: common.Address{},
				}

				// Execution specification:
				//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
				//  obtained from the Payload validation process if the payload is deemed INVALID
				s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcState, &payloadAttrbutes)
				if !syncing {
					// Execution specification:
					//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
					//  obtained from the Payload validation process if the payload is deemed INVALID
					// Note: SYNCING/ACCEPTED is acceptable here as long as the block produced after this test is produced successfully
					s.ExpectAnyPayloadStatus(Syncing, Accepted, Invalid)
				} else {
					// At this moment the response should be SYNCING
					s.ExpectPayloadStatus(Syncing)

					// When we send the previous payload, the client must now be capable of determining that the invalid payload is actually invalid
					p := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestExecutedPayload)
					p.ExpectStatus(Valid)

					/*
						// Another option here could be to send an fcU to the previous payload,
						// but this does not seem like something the CL would do.
						s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
							HeadBlockHash:      previousPayload.BlockHash,
							SafeBlockHash:      previousPayload.BlockHash,
							FinalizedBlockHash: previousPayload.BlockHash,
						}, nil)
						s.ExpectPayloadStatus(Valid)
					*/

					q := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
					if payloadField == "ParentHash" {
						// There is no invalid parentHash, if this value is incorrect,
						// it is assumed that the block is missing and we need to sync.
						// ACCEPTED also valid since the CLs normally use these interchangeably
						q.ExpectStatusEither(Syncing, Accepted)
					} else {
						// Otherwise the response should be INVALID.
						q.ExpectStatus(Invalid)
					}

					// Try sending the fcU again, this time we should get the proper invalid response.
					// At this moment the response should be INVALID
					if payloadField != "ParentHash" {
						s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&fcState, nil)
						// Note: SYNCING is acceptable here as long as the block produced after this test is produced successfully
						s.ExpectAnyPayloadStatus(Syncing, Invalid)
					}
				}

				// Finally, attempt to fetch the invalid payload using the JSON-RPC endpoint
				p := t.TestEth.TestBlockByHash(alteredPayload.BlockHash)
				p.ExpectError()
			},
		})

		if syncing {
			// Send the valid payload and its corresponding forkchoiceUpdated
			r := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestExecutedPayload)
			r.ExpectStatus(Valid)

			s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
			s.ExpectPayloadStatus(Valid)

			// Add main client again to the CL Mocker
			t.CLMock.AddEngineClient(t.T, t.Client, t.MainTTD())
		}

		// Lastly, attempt to build on top of the invalid payload
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {
				alteredPayload, err := customizePayload(&t.CLMock.LatestPayloadBuilt, &CustomPayloadData{
					ParentHash: &invalidPayloadHash,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload: %v", t.TestName, err)
				}

				t.Logf("INFO (%s): Sending customized NewPayload: ParentHash %v -> %v", t.TestName, t.CLMock.LatestPayloadBuilt.ParentHash, invalidPayloadHash)
				// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
				// or SYNCING (parent payload is thrown out and also client assumes that the parent is part of canonical chain)
				// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
				// but a VALID response is incorrect.
				r := t.TestEngine.TestEngineNewPayloadV1(alteredPayload)
				r.ExpectStatusEither(Accepted, Invalid, Syncing)

			},
		})
	}
}

// Test to verify Block information available at the Eth RPC after NewPayload
func blockStatusExecPayload(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// TODO: We can send a transaction and see if we get the transaction receipt after newPayload (we should not)
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after the new payload has been broadcasted
		OnNewPayloadBroadcast: func() {
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

			s := t.TestEth.TestBlockNumber()
			s.ExpectNumber(t.CLMock.LatestFinalizedNumber.Uint64())

			p := t.TestEth.TestBlockByNumber(nil)
			p.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test to verify Block information available at the Eth RPC after new HeadBlock ForkchoiceUpdated
func blockStatusHeadBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a forkchoice with new HeadBlockHash has been broadcasted
		OnHeadBlockForkchoiceBroadcast: func() {
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})
}

// Test to verify Block information available at the Eth RPC after new SafeBlock ForkchoiceUpdated
func blockStatusSafeBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a forkchoice with new SafeBlockHash has been broadcasted
		OnSafeBlockForkchoiceBroadcast: func() {
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)
		},
	})
}

// Test to verify Block information available at the Eth RPC after new FinalizedBlock ForkchoiceUpdated
func blockStatusFinalizedBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a forkchoice with new FinalizedBlockHash has been broadcasted
		OnFinalizedBlockForkchoiceBroadcast: func() {
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test to verify Block information available after a reorg using forkchoiceUpdated
func blockStatusReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		OnGetPayload: func() {
			// Run using an alternative Payload, verify that the latest info is updated after re-org
			customRandom := common.Hash{}
			rand.Read(customRandom[:])
			customizedPayload, err := customizePayload(&t.CLMock.LatestPayloadBuilt, &CustomPayloadData{
				PrevRandao: &customRandom,
			})
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}

			// Send custom payload and fcU to it
			t.CLMock.broadcastNewPayload(customizedPayload)
			t.CLMock.broadcastForkchoiceUpdated(&ForkchoiceStateV1{
				HeadBlockHash:      customizedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil)

			// Verify the client is serving the latest HeadBlock
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(customizedPayload.BlockHash)

		},
		OnHeadBlockForkchoiceBroadcast: func() {
			// At this point, we have re-org'd to the payload that the CLMocker was originally planning to send,
			// verify that the client is serving the latest HeadBlock.
			r := t.TestEth.TestHeaderByNumber(nil)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test that performing a re-org back into a previous block of the canonical chain does not produce errors and the chain
// is still capable of progressing.
func reorgBack(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	// We are going to reorg back to this previous hash several times
	previousHash := t.CLMock.LatestForkchoice.HeadBlockHash

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{
		OnHeadBlockForkchoiceBroadcast: func() {
			// Send a fcU with the HeadBlockHash pointing back to the previous block
			forkchoiceUpdatedBack := ForkchoiceStateV1{
				HeadBlockHash:      previousHash,
				SafeBlockHash:      previousHash,
				FinalizedBlockHash: previousHash,
			}

			// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&forkchoiceUpdatedBack, nil)
			r.ExpectNoError()
		},
	})

	// Verify that the client is pointing to the latest payload sent
	r := t.TestEth.TestBlockByNumber(nil)
	r.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

}

// Test transaction status after a forkchoiceUpdated re-orgs to an alternative hash where a transaction is not present
func transactionReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// Create transactions that modify the state in order to check after the reorg.
	var (
		txCount            = 5
		sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
	)

	for i := 0; i < txCount; i++ {
		var (
			noTxnPayload ExecutableDataV1
			tx           *types.Transaction
		)
		// Generate two payloads, one with the transaction and the other one without it
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// At this point we have not broadcast the transaction,
				// therefore any payload we get should not contain any
				t.CLMock.getNextPayloadID()
				t.CLMock.getNextPayload()
				noTxnPayload = t.CLMock.LatestPayloadBuilt
				if len(noTxnPayload.Transactions) != 0 {
					t.Fatalf("FAIL (%s): Empty payload contains transactions: %v", t.TestName, noTxnPayload)
				}

				// At this point we can broadcast the transaction and it will be included in the next payload
				// Data is the key where a `1` will be stored
				data := common.LeftPadBytes([]byte{byte(i)}, 32)
				t.Logf("transactionReorg, i=%v, data=%v\n", i, data)
				tx = t.sendNextTransaction(t.Engine, sstoreContractAddr, big0, data)

				// Get the receipt
				receipt, _ := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
				if receipt != nil {
					t.Fatalf("FAIL (%s): Receipt obtained before tx included in block: %v", t.TestName, receipt)
				}
			},
			OnGetPayload: func() {
				// Check that indeed the payload contains the transaction
				if !TransactionInPayload(&t.CLMock.LatestPayloadBuilt, tx) {
					t.Fatalf("FAIL (%s): Payload built does not contain the transaction: %v", t.TestName, t.CLMock.LatestPayloadBuilt)
				}
			},
			OnHeadBlockForkchoiceBroadcast: func() {
				// Transaction is now in the head of the canonical chain, re-org and verify it's removed
				// Get the receipt
				_, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to obtain transaction receipt: %v", t.TestName, err)
				}

				if noTxnPayload.ParentHash != t.CLMock.LatestPayloadBuilt.ParentHash {
					t.Fatalf("FAIL (%s): Incorrect parent hash for payloads: %v != %v", t.TestName, noTxnPayload.ParentHash, t.CLMock.LatestPayloadBuilt.ParentHash)
				}
				if noTxnPayload.BlockHash == t.CLMock.LatestPayloadBuilt.BlockHash {
					t.Fatalf("FAIL (%s): Incorrect hash for payloads: %v == %v", t.TestName, noTxnPayload.BlockHash, t.CLMock.LatestPayloadBuilt.BlockHash)
				}

				r := t.TestEngine.TestEngineNewPayloadV1(&noTxnPayload)
				r.ExpectStatus(Valid)

				s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
					HeadBlockHash:      noTxnPayload.BlockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil)
				s.ExpectPayloadStatus(Valid)

				p := t.TestEth.TestBlockByNumber(nil)
				p.ExpectHash(noTxnPayload.BlockHash)

				reorgReceipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
				if reorgReceipt != nil {
					t.Fatalf("FAIL (%s): Receipt was obtained when the tx had been re-org'd out: %v", t.TestName, reorgReceipt)
				}

				// Re-org back
				t.CLMock.broadcastForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil)
			},
		})

	}

}

// Reorg to a Sidechain using ForkchoiceUpdated
func sidechainReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// Produce two payloads, send fcU with first payload, check transaction outcome, then reorg, check transaction outcome again

	// This single transaction will change its outcome based on the payload
	tx := t.sendNextTransaction(t.Engine, prevRandaoContractAddr, big0, nil)
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativePrevRandao := common.Hash{}
			rand.Read(alternativePrevRandao[:])

			r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice,
				&PayloadAttributesV1{
					Timestamp:             t.CLMock.LatestFinalizedHeader.Time + 1,
					PrevRandao:            alternativePrevRandao,
					SuggestedFeeRecipient: t.CLMock.NextFeeRecipient,
				})
			r.ExpectNoError()

			time.Sleep(PayloadProductionClientDelay)

			alternativePayload, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), r.Response.PayloadID)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not get alternative payload: %v", t.TestName, err)
			}
			if len(alternativePayload.Transactions) == 0 {
				t.Fatalf("FAIL (%s): alternative payload does not contain the prevRandao opcode tx", t.TestName)
			}

			s := t.TestEngine.TestEngineNewPayloadV1(&alternativePayload)
			s.ExpectStatus(Valid)

			// We sent the alternative payload, fcU to it
			p := t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
				HeadBlockHash:      alternativePayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil)
			p.ExpectPayloadStatus(Valid)

			// PrevRandao should be the alternative prevRandao we sent
			checkPrevRandaoValue(t, alternativePrevRandao, alternativePayload.Number)
		},
	})
	// The reorg actually happens after the CLMocker continues,
	// verify here that the reorg was successful
	latestBlockNum := t.CLMock.LatestFinalizedNumber.Uint64()
	checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[latestBlockNum], latestBlockNum)

}

// Re-Execute Previous Payloads
func reExecPayloads(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// How many Payloads we are going to re-execute
	var payloadReExecCount = 10

	// Create those blocks
	t.CLMock.produceBlocks(payloadReExecCount, BlockProcessCallbacks{})

	// Re-execute the payloads
	r := t.TestEth.TestBlockNumber()
	r.ExpectNoError()
	lastBlock := r.Number
	t.Logf("INFO (%s): Started re-executing payloads at block: %v", t.TestName, lastBlock)

	for i := lastBlock - uint64(payloadReExecCount) + 1; i <= lastBlock; i++ {
		payload, found := t.CLMock.ExecutedPayloadHistory[i]
		if !found {
			t.Fatalf("FAIL (%s): (test issue) Payload with index %d does not exist", i)
		}

		r := t.TestEngine.TestEngineNewPayloadV1(&payload)
		r.ExpectStatus(Valid)

	}
}

// Multiple New Payloads Extending Canonical Chain
func multipleNewCanonicalPayloads(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after a new payload has been obtained
		OnGetPayload: func() {
			payloadCount := 80
			basePayload := t.CLMock.LatestPayloadBuilt

			// Fabricate and send multiple new payloads by changing the PrevRandao field
			for i := 0; i < payloadCount; i++ {
				newPrevRandao := common.Hash{}
				rand.Read(newPrevRandao[:])
				newPayload, err := customizePayload(&basePayload, &CustomPayloadData{
					PrevRandao: &newPrevRandao,
				})
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to customize payload %v: %v", t.TestName, i, err)
				}

				r := t.TestEngine.TestEngineNewPayloadV1(newPayload)
				r.ExpectStatus(Valid)

			}
		},
	})
	// At the end the CLMocker continues to try to execute fcU with the original payload, which should not fail
}

// Out of Order Payload Execution: Secondary client should be able to set the forkchoiceUpdated to payloads received out of order
func outOfOrderPayloads(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// First prepare payloads on a first client, which will also contain multiple transactions

	// We will be also verifying that the transactions are correctly interpreted in the canonical chain,
	// prepare a random account to receive funds.
	recipient := common.Address{}
	rand.Read(recipient[:])
	amountPerTx := big.NewInt(1000)
	txPerPayload := 20
	payloadCount := 10

	t.CLMock.produceBlocks(payloadCount, BlockProcessCallbacks{
		// We send the transactions after we got the Payload ID, before the CLMocker gets the prepared Payload
		OnPayloadProducerSelected: func() {
			for i := 0; i < txPerPayload; i++ {
				t.sendNextTransaction(t.CLMock.NextBlockProducer, recipient, amountPerTx, nil)
			}
		},
	})

	expectedBalance := amountPerTx.Mul(amountPerTx, big.NewInt(int64(payloadCount*txPerPayload)))

	// Check balance on this first client
	r := t.TestEth.TestBalanceAt(recipient, nil)
	r.ExpectBalanceEqual(expectedBalance)

	// Start a second client to send forkchoiceUpdated + newPayload out of order
	allClients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain all client types", t.TestName)
	}

	secondaryTestEngineClients := make([]*TestEngineClient, len(allClients))

	for i, client := range allClients {
		_, c, err := t.StartClient(client.Name, t.ClientParams, t.MainTTD())
		secondaryTestEngineClients[i] = NewTestEngineClient(t, c)

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to start client (%v): %v", t.TestName, client, err)
		}
		// Send the forkchoiceUpdated with the LatestExecutedPayload hash, we should get SYNCING back
		fcU := ForkchoiceStateV1{
			HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
			SafeBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
			FinalizedBlockHash: t.CLMock.LatestExecutedPayload.BlockHash,
		}

		r := secondaryTestEngineClients[i].TestEngineForkchoiceUpdatedV1(&fcU, nil)
		r.ExpectPayloadStatus(Syncing)
		r.ExpectLatestValidHash(nil)
		r.ExpectNoValidationError()

		// Send all the payloads in the opposite order
		for k := t.CLMock.LatestExecutedPayload.Number; k > 0; k-- {
			payload := t.CLMock.ExecutedPayloadHistory[k]

			if k > 1 {
				r := secondaryTestEngineClients[i].TestEngineNewPayloadV1(&payload)
				r.ExpectStatusEither(Accepted, Syncing)
				r.ExpectLatestValidHash(nil)
				r.ExpectNoValidationError()
			} else {
				r := secondaryTestEngineClients[i].TestEngineNewPayloadV1(&payload)
				r.ExpectStatus(Valid)
				r.ExpectLatestValidHash(&payload.BlockHash)

			}
		}
	}
	// Add the clients to the CLMocker
	for _, tec := range secondaryTestEngineClients {
		t.CLMock.AddEngineClient(t.T, tec.Engine.Client, t.MainTTD())
	}

	// Produce a single block on top of the canonical chain, all clients must accept this
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	for _, ec := range secondaryTestEngineClients {
		// At this point we should have our funded account balance equal to the expected value.
		r := NewTestEthClient(t, ec.Engine.Eth).TestBalanceAt(recipient, nil)
		r.ExpectBalanceEqual(expectedBalance)

		ec.Close()
	}
}

// Send a valid payload on a client that is currently SYNCING
func validPayloadFcUSyncingClient(t *TestEnv) {
	var (
		secondaryClient *hivesim.Client
		previousPayload ExecutableDataV1
	)
	{
		// To allow sending the primary engine client into SYNCING state, we need a secondary client to guide the payload creation
		var err error
		secondaryClient, _, err = t.StartClient(t.Client.Type, t.ClientParams, t.MainTTD())
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		t.CLMock.AddEngineClient(t.T, secondaryClient, t.MainTTD())
	}

	// Wait until TTD is reached by all clients
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	// Disconnect the first engine client from the CL Mocker and produce a block
	t.CLMock.RemoveEngineClient(t.Client)
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	previousPayload = t.CLMock.LatestPayloadBuilt

	// Send the fcU to set it to syncing mode
	r := t.TestEngine.TestEngineForkchoiceUpdatedV1(&t.CLMock.LatestForkchoice, nil)
	r.ExpectPayloadStatus(Syncing)

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Send the new payload from the second client to the first, it won't be able to validate it
			r := t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestPayloadBuilt)
			r.ExpectStatusEither(Accepted, Syncing)

			// Send the forkchoiceUpdated with a reference to the valid payload on the SYNCING client.
			s := t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, &PayloadAttributesV1{
				Timestamp:             t.CLMock.LatestPayloadBuilt.Timestamp + 1,
				PrevRandao:            common.Hash{},
				SuggestedFeeRecipient: common.Address{},
			})
			s.ExpectPayloadStatus(Syncing)

			// Send the previous payload to be able to continue
			p := t.TestEngine.TestEngineNewPayloadV1(&previousPayload)
			p.ExpectStatus(Valid)

			// Send the new payload again

			p = t.TestEngine.TestEngineNewPayloadV1(&t.CLMock.LatestPayloadBuilt)
			p.ExpectStatus(Valid)

			s = t.TestEngine.TestEngineForkchoiceUpdatedV1(&ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				SafeBlockHash:      t.CLMock.LatestPayloadBuilt.BlockHash,
				FinalizedBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
			}, nil)
			s.ExpectPayloadStatus(Valid)

		},
	})

	// Add the secondary client again to the CL Mocker
	t.CLMock.AddEngineClient(t.T, t.Client, t.MainTTD())

	t.CLMock.RemoveEngineClient(secondaryClient)
}

// Fee Recipient Tests
func suggestedFeeRecipient(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// Amount of transactions to send
	txCount := 20

	// Verify that, in a block with transactions, fees are accrued by the suggestedFeeRecipient
	feeRecipient := common.Address{}
	rand.Read(feeRecipient[:])

	// Send multiple transactions
	for i := 0; i < txCount; i++ {
		t.sendNextTransaction(t.Engine, vaultAccountAddr, big0, nil)
	}
	// Produce the next block with the fee recipient set
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	// Calculate the fees and check that they match the balance of the fee recipient
	r := t.TestEth.TestBlockByNumber(nil)
	r.ExpectTransactionCountEqual(txCount)
	r.ExpectCoinbase(feeRecipient)
	blockIncluded := r.Block

	feeRecipientFees := big.NewInt(0)
	for _, tx := range blockIncluded.Transactions() {
		effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain EffectiveGasTip: %v", t.TestName, err)
		}
		receipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
		if err != nil {
			t.Fatalf("FAIL (%s): unable to obtain receipt: %v", t.TestName, err)
		}
		feeRecipientFees = feeRecipientFees.Add(feeRecipientFees, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
	}

	s := t.TestEth.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

	// Produce another block without txns and get the balance again
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	s = t.TestEth.TestBalanceAt(feeRecipient, nil)
	s.ExpectBalanceEqual(feeRecipientFees)

}

// TODO: Do a PENDING block suggestedFeeRecipient

func checkPrevRandaoValue(t *TestEnv, expectedPrevRandao common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	r := t.TestEth.TestStorageAt(prevRandaoContractAddr, storageKey, nil)
	r.ExpectStorageEqual(expectedPrevRandao)

}

// PrevRandao Opcode tests
func prevRandaoOpcodeTx(t *TestEnv) {
	// We need to send PREVRANDAO opcode transactions in PoW and particularly in the block where the TTD is reached.
	ttdReached := make(chan interface{})

	// Try to send many transactions before PoW transition to guarantee at least one enters in the block
	go func(t *TestEnv) {
		for {
			t.sendNextTransaction(t.Engine, prevRandaoContractAddr, big0, nil)

			select {
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout while sending PREVRANDAO opcode transactions: %v")
			case <-ttdReached:
				return
			case <-time.After(time.Second / 10):
			}
		}
	}(t)
	t.CLMock.waitForTTD()
	close(ttdReached)

	// Ideally all blocks up until TTD must have a DIFFICULTY opcode tx in it
	r := t.TestEth.TestBlockNumber()
	r.ExpectNoError()
	ttdBlockNumber := r.Number

	// Start
	for i := uint64(ttdBlockNumber); i <= ttdBlockNumber; i++ {
		// First check that the block actually contained the transaction
		r := t.TestEth.TestBlockByNumber(big.NewInt(int64(i)))
		r.ExpectTransactionCountGreaterThan(0)

		storageKey := common.Hash{}
		storageKey[31] = byte(i)
		s := t.TestEth.TestStorageAt(prevRandaoContractAddr, storageKey, nil)
		s.ExpectBigIntStorageEqual(big.NewInt(2))

	}

	// Send transactions now past TTD, the value of the storage in these blocks must match the prevRandao value
	var (
		txCount        = 10
		currentTxIndex = 0
		txs            = make([]*types.Transaction, 0)
	)
	t.CLMock.produceBlocks(txCount, BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			tx := t.sendNextTransaction(t.Engine, prevRandaoContractAddr, big0, nil)
			txs = append(txs, tx)
			currentTxIndex++
		},
		OnHeadBlockForkchoiceBroadcast: func() {
			// Check the transaction tracing, which is client specific
			expectedPrevRandao := t.CLMock.PrevRandaoHistory[t.CLMock.LatestFinalizedHeader.Number.Uint64()+1]
			if err := debugPrevRandaoTransaction(t.Engine.Ctx(), t.RPC, t.Engine.Client.Type, txs[currentTxIndex-1],
				&expectedPrevRandao); err != nil {
				t.Fatalf("FAIL (%s): Error during transaction tracing: %v", t.TestName, err)
			}
		},
	})

	lastBlockNumber, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	for i := ttdBlockNumber + 1; i <= lastBlockNumber; i++ {
		checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[i], i)
	}

}

// Client Sync tests
func postMergeSync(t *TestEnv) {
	// Launch another client after the PoS transition has happened in the main client.
	// Sync should eventually happen without issues.
	t.CLMock.waitForTTD()

	// Produce some blocks
	t.CLMock.produceBlocks(10, BlockProcessCallbacks{})

	allClients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain all client types", t.TestName)
	}
	// Set the Bootnode
	enode, err := t.Engine.EnodeURL()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain bootnode: %v", t.TestName, err)
	}
	newParams := t.ClientParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))
	newParams = newParams.Set("HIVE_MINER", "")

	for _, client := range allClients {
		c, ec, err := t.StartClient(client.Name, newParams, t.MainTTD())
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to start client (%v): %v", t.TestName, client, err)
		}
		// Add engine client and broadcast to it the latest forkchoice
		t.CLMock.AddEngineClient(t.T, c, t.MainTTD())
	syncLoop:
		for {
			select {
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Test timeout", t.TestName)
			default:
			}

			// CL continues building blocks on top of the PoS chain
			t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

			// When the main client syncs, the test passes
			latestHeader, err := ec.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to obtain latest header: %v", t.TestName, err)
			}
			if t.CLMock.LatestFinalizedHeader != nil && latestHeader.Hash() == t.CLMock.LatestFinalizedHeader.Hash() {
				t.Logf("INFO (%v): Client (%v) is now synced to latest PoS block: %v", t.TestName, c.Container, latestHeader.Hash())
				break syncLoop
			}
		}
	}
}
