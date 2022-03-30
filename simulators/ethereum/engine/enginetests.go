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
		Name: "Pre-TTD ForkchoiceUpdated After PoS Switch",
		Run:  preTTDFinalizedBlockHash,
		TTD:  2,
	},
	{
		Name: "Bad Hash on NewPayload",
		Run:  badHashOnExecPayload,
	},
	{
		Name: "ParentHash==BlockHash on NewPayload",
		Run:  parentHashOnExecPayload,
	},
	{
		Name: "Invalid ParentHash NewPayload",
		Run:  invalidPayloadTestCaseGen("ParentHash"),
	},
	{
		Name: "Invalid StateRoot NewPayload",
		Run:  invalidPayloadTestCaseGen("StateRoot"),
	},
	{
		Name: "Invalid ReceiptsRoot NewPayload",
		Run:  invalidPayloadTestCaseGen("ReceiptsRoot"),
	},
	{
		Name: "Invalid Number NewPayload",
		Run:  invalidPayloadTestCaseGen("Number"),
	},
	{
		Name: "Invalid GasLimit NewPayload",
		Run:  invalidPayloadTestCaseGen("GasLimit"),
	},
	{
		Name: "Invalid GasUsed NewPayload",
		Run:  invalidPayloadTestCaseGen("GasUsed"),
	},
	{
		Name: "Invalid Timestamp NewPayload",
		Run:  invalidPayloadTestCaseGen("Timestamp"),
	},
	{
		Name: "Invalid PrevRandao NewPayload",
		Run:  invalidPayloadTestCaseGen("PrevRandao"),
	},
	{
		Name: "Invalid Incomplete Transactions NewPayload",
		Run:  invalidPayloadTestCaseGen("RemoveTransaction"),
	},
	{
		Name: "Invalid Transaction Signature NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Signature"),
	},
	{
		Name: "Invalid Transaction Nonce NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Nonce"),
	},
	{
		Name: "Invalid Transaction GasPrice NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/GasPrice"),
	},
	{
		Name: "Invalid Transaction Gas NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Gas"),
	},
	{
		Name: "Invalid Transaction Value NewPayload",
		Run:  invalidPayloadTestCaseGen("Transaction/Value"),
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

	// Transaction Reorg using Engine API
	{
		Name: "Transaction Reorg",
		Run:  transactionReorg,
	},
	{
		Name: "Sidechain Reorg",
		Run:  sidechainReorg,
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

	// Exchange Transition Configuration tests
	{
		Name: "Exchange Transition Configuration PreMerge",
		Run:  exTransitionConfigPreMerge,
		TTD:  1000000,
	},
	{
		Name: "Exchange Transition Configuration PostMerge",
		Run:  exTransitionConfigPostMerge,
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
	fcResp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceState, nil)
	// Execution specification:
	// {payloadStatus: {status: INVALID_TERMINAL_BLOCK, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
	// either obtained from the Payload validation process or as a result of validating a PoW block referenced by forkchoiceState.headBlockHash
	if err != nil {
		t.Fatalf("FAIL (%s): ForkchoiceUpdated under PoW rule returned error (Expected INVALID_TERMINAL_BLOCK): %v, %v", t.TestName, err)
	}
	if fcResp.PayloadStatus.Status != "INVALID_TERMINAL_BLOCK" {
		t.Fatalf("INFO (%v): Incorrect EngineForkchoiceUpdatedV1 response for invalid PoW parent (Expected PayloadStatus.Status=INVALID_TERMINAL_BLOCK): %v", t.TestName, fcResp.PayloadStatus.Status)
	}
	if fcResp.PayloadStatus.LatestValidHash != nil {
		t.Fatalf("INFO (%v): Incorrect EngineForkchoiceUpdatedV1 response for invalid PoW parent (Expected PayloadStatus.LatestValidHash==nil): %v", t.TestName, fcResp.PayloadStatus)
	}
	// ValidationError is not validated since it can be either null or a string message
}

// Invalid GetPayload Under PoW: Client must reject GetPayload directives under PoW.
func invalidGetPayloadUnderPoW(t *TestEnv) {
	// We start in PoW and try to get an invalid Payload, which should produce an error but nothing should be disrupted.
	payloadResp, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), &PayloadID{1, 2, 3, 4, 5, 6, 7, 8})
	if err == nil {
		t.Fatalf("FAIL (%s): GetPayloadV1 accepted under PoW rule: %v", t.TestName, payloadResp)
	}

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

	newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), hashedPayload)

	// Execution specification:
	// {status: INVALID_TERMINAL_BLOCK, latestValidHash: null, validationError: errorMessage | null}
	// if terminal block conditions are not satisfied
	if err != nil {
		t.Fatalf("FAIL (%s): EngineNewPayloadV1 under PoW rule returned error (Expected INVALID_TERMINAL_BLOCK): %v", t.TestName, err)
	}
	if newPayloadResp.Status != "INVALID_TERMINAL_BLOCK" {
		t.Fatalf("FAIL (%s): Incorrect EngineNewPayloadV1 response for invalid PoW parent (Expected Status=INVALID_TERMINAL_BLOCK): %v", t.TestName, newPayloadResp.Status)
	}
	if newPayloadResp.LatestValidHash != nil {
		t.Fatalf("FAIL (%s): Incorrect EngineNewPayloadV1 response for invalid PoW parent (Expected LatestValidHash==nil): %v", t.TestName, newPayloadResp.LatestValidHash)
	}
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
			forkchoiceStateUnknownSafeHash := ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
				SafeBlockHash:      randomSafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}
			// Execution specification:
			// - This value MUST be either equal to or an ancestor of headBlockHash
			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownSafeHash, nil)
			if err == nil {
				t.Fatalf("FAIL (%s): No error on forkchoiceUpdated with unknown SafeBlockHash: %v", t.TestName, resp)
			}

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
			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, nil)
			if err == nil {
				t.Fatalf("FAIL (%s): No error on forkchoiceUpdated with unknown FinalizedBlockHash: %v", t.TestName, resp)
			}

			// Test again using PayloadAttributes, should also return INVALID and no PayloadID
			payloadAttr := PayloadAttributesV1{
				Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
				PrevRandao:            common.Hash{},
				SuggestedFeeRecipient: common.Address{},
			}
			resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, &payloadAttr)
			if err == nil {
				t.Fatalf("FAIL (%s): No error on forkchoiceUpdated with unknown FinalizedBlockHash: %v", t.TestName, resp)
			}

		},
	})

}

// Verify that an unknown hash at HeadBlock in the forkchoice results in client returning "SYNCING" state
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

	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownHeadHash, nil)
	if err != nil {
		t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown HeadBlockHash: %v", t.TestName, err)
	}
	// Execution specification::
	// - {payloadStatus: {status: SYNCING, latestValidHash: null, validationError: null}, payloadId: null}
	//   if forkchoiceState.headBlockHash references an unknown payload or a payload that can't be validated
	//   because requisite data for the validation is missing
	if resp.PayloadStatus.Status != "SYNCING" {
		t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown HeadBlockHash is not SYNCING: %v", t.TestName, resp)
	}

	// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
	payloadAttr := PayloadAttributesV1{
		Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
		PrevRandao:            common.Hash{},
		SuggestedFeeRecipient: common.Address{},
	}
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownHeadHash, &payloadAttr)
	if err != nil {
		t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown HeadBlockHash + PayloadAttributes: %v", t.TestName, err)
	}
	if resp.PayloadStatus.Status != "SYNCING" {
		t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown HeadBlockHash is not SYNCING: %v, %v", t.TestName, resp)
	}
	if resp.PayloadID != nil {
		t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown HeadBlockHash contains PayloadID: %v, %v", t.TestName, resp)
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
	forkchoiceStateGenesisHash := ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}
	t.Logf("INFO (%v): Sending genesis block forkchoiceUpdated: %v", t.TestName, gblock.Hash())
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateGenesisHash, nil)

	/* TBD: Behavior on this edge-case is undecided, as behavior of the Execution client
	 		if not defined on re-orgs to a point before the latest finalized block.

	if err == nil {
		t.Fatalf("FAIL (%s): No error forkchoiceUpdated with genesis: %v, %v", t.TestName, err, resp)
	}
	*/

	t.Logf("INFO (%v): Sending latest forkchoice again: %v", t.TestName, t.CLMock.LatestForkchoice.FinalizedBlockHash)
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
	if err != nil {
		t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
	}
	if resp.PayloadStatus.Status != "VALID" {
		t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with LatestForkchoice is not VALID: %v, %v", t.TestName, resp)
	}
}

// Corrupt the hash of a valid payload, client should reject the payload
func badHashOnExecPayload(t *TestEnv) {
	// Wait until TTD is reached by this client
	t.CLMock.waitForTTD()

	// Produce blocks before starting the test
	t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

	var invalidPayloadHash common.Hash

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload := t.CLMock.LatestPayloadBuilt
			invalidPayloadHash = alteredPayload.BlockHash
			invalidPayloadHash[common.HashLength-1] = byte(255 - invalidPayloadHash[common.HashLength-1])
			alteredPayload.BlockHash = invalidPayloadHash
			newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &alteredPayload)
			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			if err != nil {
				t.Fatalf("FAIL (%s): Incorrect block hash in execute payload resulted in error: %v", t.TestName, err)
			}
			if newPayloadResp.Status != "INVALID_BLOCK_HASH" {
				t.Fatalf("FAIL (%s): Incorrect block hash in execute payload returned unexpected status (exp INVALID_BLOCK_HASH): %v", t.TestName, newPayloadResp.Status)
			}
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
			resp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), alteredPayload)
			if err != nil {
				t.Fatalf("FAIL (%s): Valid EngineNewPayload on top of Invalid Payload was rejected: %v", t.TestName, err)
			}
			// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
			// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
			// but a VALID response is incorrect.
			if resp.Status == "VALID" {
				t.Fatalf("FAIL (%s): Unexpected response on valid payload on top of invalid payload: %v", t.TestName, resp)
			}
		},
	})

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
			newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &alteredPayload)
			// Execution specification::
			// - {status: INVALID_BLOCK_HASH, latestValidHash: null, validationError: null} if the blockHash validation has failed
			if err != nil {
				t.Fatalf("FAIL (%s): Incorrect block hash in execute payload resulted in error: %v", t.TestName, err)
			}
			if newPayloadResp.Status != "INVALID_BLOCK_HASH" {
				t.Fatalf("FAIL (%s): Incorrect block hash in execute payload returned unexpected status (exp INVALID_BLOCK_HASH): %v", t.TestName, newPayloadResp.Status)
			}
		},
	})

}

// Generate test cases for each field of NewPayload, where the payload contains a single invalid field and a valid hash.
func invalidPayloadTestCaseGen(payloadField string) func(*TestEnv) {
	return func(t *TestEnv) {
		// Wait until TTD is reached by this client
		t.CLMock.waitForTTD()

		txFunc := func() {
			// Function to send at least one transaction each block produced
			// Send the transaction to the prevRandaoContractAddr
			tx := t.makeNextTransaction(prevRandaoContractAddr, big1, nil)
			if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
				t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
			}
		}

		// Produce blocks before starting the test
		t.CLMock.produceBlocks(5, BlockProcessCallbacks{
			// Make sure at least one transaction is included in each block
			OnPayloadProducerSelected: txFunc,
		})

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
				newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), alteredPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Incorrect %v in EngineNewPayload was rejected: %v", t.TestName, payloadField, err)
				}
				// Depending on the field we modified, we expect a different status
				var expectedState string
				var expectedLatestValidHash *common.Hash = nil
				if payloadField == "ParentHash" {
					// Execution specification::
					// {status: ACCEPTED, latestValidHash: null, validationError: null} if the following conditions are met:
					//  - the blockHash of the payload is valid
					//  - the payload doesn't extend the canonical chain
					//  - the payload hasn't been fully validated
					expectedState = "ACCEPTED"
				} else {
					expectedState = "INVALID"
					expectedLatestValidHash = &alteredPayload.ParentHash
				}

				t.Logf("INFO (%v): Invalid payload response: %v", t.TestName, newPayloadResp)
				if newPayloadResp.Status != expectedState {
					t.Fatalf("FAIL (%s): EngineNewPayload with reference to invalid payload returned incorrect state: %v!=%v", t.TestName, newPayloadResp.Status, expectedState)
				}
				if newPayloadResp.LatestValidHash != nil && expectedLatestValidHash != nil {
					// Both expected and received are different from nil, therefore their values must be equal.
					if *newPayloadResp.LatestValidHash != *expectedLatestValidHash {
						t.Fatalf("FAIL (%s): EngineNewPayload with reference to invalid payload returned incorrect LatestValidHash: %v!=%v", t.TestName, newPayloadResp.LatestValidHash, expectedLatestValidHash)
					}
				} else if newPayloadResp.LatestValidHash != expectedLatestValidHash {
					// At least one of them is equal to nil, but they also point to different locations.
					t.Fatalf("FAIL (%s): EngineNewPayload with reference to invalid payload returned incorrect LatestValidHash: %v!=%v", t.TestName, newPayloadResp.LatestValidHash, expectedLatestValidHash)
				} else {
					// The expected value and the received value both point to nil, and that's ok.
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
				fcResp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &fcState, &payloadAttrbutes)
				// Execution specification:
				//  {payloadStatus: {status: INVALID, latestValidHash: null, validationError: errorMessage | null}, payloadId: null}
				//  obtained from the Payload validation process if the payload is deemed INVALID
				if err != nil {
					t.Fatalf("FAIL (%s): ForkchoiceUpdated with reference to invalid payload resulted in error: %v", t.TestName, err)
				}
				// Note: SYNCING is acceptable here as long as the block produced after this test is produced successfully
				if fcResp.PayloadStatus.Status != "INVALID" && fcResp.PayloadStatus.Status != "SYNCING" {
					t.Fatalf("FAIL (%s): ForkchoiceUpdated with reference to invalid payload returned incorrect state: %v!=INVALID|SYNCING", t.TestName, fcResp.PayloadStatus.Status)
				}

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
				t.Logf("INFO (%s): Sending customized NewPayload: ParentHash %v -> %v", t.TestName, t.CLMock.LatestPayloadBuilt.ParentHash, invalidPayloadHash)
				resp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), alteredPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Valid EngineNewPayload on top of Invalid Payload was rejected: %v", t.TestName, err)
				}
				t.Logf("INFO (%s): NewPayload response on top of invalid payload: %v", t.TestName, resp)
				// Response status can be ACCEPTED (since parent payload could have been thrown out by the client)
				// or INVALID (client still has the payload and can verify that this payload is incorrectly building on top of it),
				// but a VALID response is incorrect.
				if resp.Status == "VALID" {
					t.Fatalf("FAIL (%s): Unexpected response on valid payload on top of invalid payload: %v", t.TestName, resp)
				}
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
			// TODO: Ideally, we would need to check that the newPayload returned VALID
			latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			// Latest block header available via Eth RPC should not have changed at this point
			if latestBlockHeader.Hash() == t.CLMock.LatestExecutedPayload.BlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header incorrect after newPayload: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

			latestBlockNumber, err := t.Eth.BlockNumber(t.Ctx())
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
			}
			// Latest block number available via Eth RPC should not have changed at this point
			if latestBlockNumber != t.CLMock.LatestFinalizedNumber.Uint64() {
				t.Fatalf("FAIL (%s): latest block number incorrect after newPayload: %d, %d", t.TestName, latestBlockNumber, t.CLMock.LatestFinalizedNumber.Uint64())
			}

			latestBlock, err := t.Eth.BlockByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}

			// Latest block available via Eth RPC should not have changed at this point
			if latestBlock.Hash() == t.CLMock.LatestExecutedPayload.BlockHash ||
				latestBlock.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				latestBlock.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
				latestBlock.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block incorrect after newPayload: %v, %v", t.TestName, latestBlock.Hash(), t.CLMock.LatestForkchoice)
			}
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

			latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				latestBlockHeader.Hash() == t.CLMock.LatestForkchoice.SafeBlockHash ||
				latestBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match HeadBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

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

			latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
				latestBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match SafeBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

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

			latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
				latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match FinalizedBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

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
			currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if currentBlockHeader.Hash() != customizedPayload.BlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match HeadBlock hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

		},
		OnHeadBlockForkchoiceBroadcast: func() {
			// At this point, we have re-org'd to the payload that the CLMocker was originally planning to send,
			// verify that the client is serving the latest HeadBlock.
			currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				currentBlockHeader.Hash() == t.CLMock.LatestForkchoice.SafeBlockHash ||
				currentBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match HeadBlock hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

		},
	})

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
				tx = t.makeNextTransaction(sstoreContractAddr, big0, data)

				// Send the transaction
				if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
					t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
				}
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

				status, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &noTxnPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Error sending no-txn payload: %v", t.TestName, err)
				}
				if status.Status != "VALID" {
					t.Fatalf("FAIL (%s): Invalid status on sending no-txn payload: %v", t.TestName, status.Status)
				}
				resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &ForkchoiceStateV1{
					HeadBlockHash:      noTxnPayload.BlockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil)
				if err != nil {
					t.Fatalf("FAIL (%s): Error during forkchoiceUpdated to no-txn payload: %v", t.TestName, err)
				}
				if resp.PayloadStatus.Status != "VALID" {
					t.Fatalf("FAIL (%s): Invalid status on forkchoiceUpdated for no-txn payload: %v", t.TestName, resp.PayloadStatus.Status)
				}

				latestBlock, err := t.Eth.BlockByNumber(t.Engine.Ctx(), nil)
				if err != nil {
					t.Fatalf("FAIL (%s): Error getting latest block after no-txn payload: %v", t.TestName, err)
				}
				if latestBlock.Hash() != noTxnPayload.BlockHash {
					t.Fatalf("FAIL (%s): Incorrect block after re-org: %v != %v", t.TestName, latestBlock.Hash(), noTxnPayload.BlockHash)
				}
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
	tx := t.makeNextTransaction(prevRandaoContractAddr, big0, nil)
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativePrevRandao := common.Hash{}
			rand.Read(alternativePrevRandao[:])

			payloadAttributes := PayloadAttributesV1{
				Timestamp:             t.CLMock.LatestFinalizedHeader.Time + 1,
				PrevRandao:            alternativePrevRandao,
				SuggestedFeeRecipient: t.CLMock.NextFeeRecipient,
			}

			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, &payloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
			}
			time.Sleep(time.Second)
			alternativePayload, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), resp.PayloadID)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not get alternative payload: %v", t.TestName, err)
			}
			if len(alternativePayload.Transactions) == 0 {
				t.Fatalf("FAIL (%s): alternative payload does not contain the prevRandao opcode tx", t.TestName)
			}
			alternativePayloadStatus, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &alternativePayload)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send alternative payload: %v", t.TestName, err)
			}
			if alternativePayloadStatus.Status != "VALID" {
				t.Fatalf("FAIL (%s): Alternative payload response returned Status!=VALID: %v", t.TestName, alternativePayloadStatus)
			}
			// We sent the alternative payload, fcU to it
			alternativeFcU := ForkchoiceStateV1{
				HeadBlockHash:      alternativePayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}
			alternativeFcUResp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &alternativeFcU, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send alternative fcU: %v", t.TestName, err)
			}
			if alternativeFcUResp.PayloadStatus.Status != "VALID" {
				t.Fatalf("FAIL (%s): Alternative fcU response returned Status!=VALID: %v", t.TestName, alternativeFcUResp)
			}

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
	lastBlock, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): Started re-executing payloads at block: %v", t.TestName, lastBlock)

	for i := lastBlock - uint64(payloadReExecCount) + 1; i <= lastBlock; i++ {
		payload, found := t.CLMock.ExecutedPayloadHistory[i]
		if !found {
			t.Fatalf("FAIL (%s): (test issue) Payload with index %d does not exist", i)
		}
		newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &payload)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to re-execute valid payload: %v", err)
		}
		if newPayloadResp.Status != "VALID" {
			t.Fatalf("FAIL (%s): Unexpected status after re-execute valid payload: %v", newPayloadResp)
		}
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
				newPayloadResp, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), newPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to send new valid payload extending canonical chain: %v", err)
				}
				if newPayloadResp.Status != "VALID" {
					t.Fatalf("FAIL (%s): Unexpected status after trying to send new valid payload extending canonical chain: %v", newPayloadResp)
				}
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
	payloadCount := 2

	t.CLMock.produceBlocks(payloadCount, BlockProcessCallbacks{
		// We send the transactions after we got the Payload ID, before the CLMocker gets the prepared Payload
		OnPayloadProducerSelected: func() {
			for i := 0; i < txPerPayload; i++ {
				tx := t.makeNextTransaction(recipient, amountPerTx, nil)
				err := t.Eth.SendTransaction(t.Ctx(), tx)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
				}
			}
		},
	})

	expectedBalance := amountPerTx.Mul(amountPerTx, big.NewInt(int64(payloadCount*txPerPayload)))

	// Check balance on this first client
	bal, err := t.Eth.BalanceAt(t.Ctx(), recipient, nil)
	if err != nil {
		t.Fatalf("FAIL (%s): Error while getting balance of funded account: %v", t.TestName, err)
	}
	if bal.Cmp(expectedBalance) != 0 {
		t.Fatalf("FAIL (%s): Incorrect balance after payload execution: %v!=%v", t.TestName, bal, expectedBalance)
	}

	// Start a second client to send forkchoiceUpdated + newPayload out of order
	allClients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain all client types", t.TestName)
	}

	secondaryEngineClients := make([]*EngineClient, len(allClients))

	for i, client := range allClients {
		_, ec, err := t.StartClient(client, t.ClientParams, t.MainTTD())
		secondaryEngineClients[i] = ec

		if err != nil {
			t.Fatalf("FAIL (%s): Unable to start client (%v): %v", t.TestName, client, err)
		}
		// Send the forkchoiceUpdated with the LatestExecutedPayload hash, we should get SYNCING back
		fcU := ForkchoiceStateV1{
			HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
			SafeBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
			FinalizedBlockHash: t.CLMock.LatestExecutedPayload.BlockHash,
		}
		fcResp, err := ec.EngineForkchoiceUpdatedV1(ec.Ctx(), &fcU, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Error while sending EngineForkchoiceUpdatedV1: %v", t.TestName, err)
		}
		if fcResp.PayloadStatus.Status != "SYNCING" {
			t.Fatalf("FAIL (%s): Incorrect PayloadStatus.Status!=SYNCING: %v", t.TestName, fcResp.PayloadStatus.Status)
		}
		if fcResp.PayloadStatus.LatestValidHash != nil {
			t.Fatalf("FAIL (%s): Incorrect PayloadStatus.LatestValidHash!=null: %v", t.TestName, fcResp.PayloadStatus.LatestValidHash)
		}
		if fcResp.PayloadStatus.ValidationError != nil {
			t.Fatalf("FAIL (%s): Incorrect PayloadStatus.ValidationError!=null: %v", t.TestName, fcResp.PayloadStatus.ValidationError)
		}

		// Send all the payloads in the opposite order
		for i := t.CLMock.LatestExecutedPayload.Number; i > 0; i-- {
			payload := t.CLMock.ExecutedPayloadHistory[i]
			payloadResp, err := ec.EngineNewPayloadV1(ec.Ctx(), &payload)
			if err != nil {
				t.Fatalf("FAIL (%s): Error while sending EngineNewPayloadV1: %v, %v", t.TestName, err, payload)
			}
			if i > 1 {
				if payloadResp.Status != "ACCEPTED" && payloadResp.Status != "SYNCING" {
					t.Fatalf("FAIL (%s): Incorrect Status!=ACCEPTED|SYNCING (Payload Number=%v): %v", t.TestName, i, payloadResp.Status)
				}
				if payloadResp.LatestValidHash != nil {
					t.Fatalf("FAIL (%s): Incorrect LatestValidHash!=nil: %v", t.TestName, payloadResp.LatestValidHash)
				}
				if payloadResp.ValidationError != nil {
					t.Fatalf("FAIL (%s): Incorrect ValidationError!=nil: %v", t.TestName, payloadResp.ValidationError)
				}

			} else {
				// On the Payload 1, the payload is VALID since we have the complete information to validate the chain
				if payloadResp.Status != "VALID" {
					t.Fatalf("FAIL (%s): Incorrect Status!=ACCEPTED (Payload Number=%v): %v", t.TestName, i, payloadResp.Status)
				}
				if payloadResp.LatestValidHash == nil {
					t.Fatalf("FAIL (%s): Incorrect LatestValidHash==nil (Payload Number=%v): %v", t.TestName, i, payloadResp.LatestValidHash)
				}
				if *payloadResp.LatestValidHash != payload.BlockHash {
					t.Fatalf("FAIL (%s): Incorrect LatestValidHash (Payload Number=%v): %v!=%v", t.TestName, i, *payloadResp.LatestValidHash, payload.BlockHash)
				}
			}
		}
	}
	// Add the clients to the CLMocker
	for _, ec := range secondaryEngineClients {
		t.CLMock.AddEngineClient(t.T, ec.Client, t.MainTTD())
	}

	// Produce a single block on top of the canonical chain, all clients must accept this
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	for _, ec := range secondaryEngineClients {
		// At this point we should have our funded account balance equal to the expected value.
		bal, err := ec.Eth.BalanceAt(ec.Ctx(), recipient, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Error while getting balance of funded account: %v", t.TestName, err)
		}
		if bal.Cmp(expectedBalance) != 0 {
			t.Fatalf("FAIL (%s): Incorrect balance after payload execution: %v!=%v", t.TestName, bal, expectedBalance)
		}
		ec.Close()
	}
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
		tx := t.makeNextTransaction(vaultAccountAddr, big0, nil)
		// Empty self tx
		for {
			err := t.Eth.SendTransaction(t.Ctx(), tx)
			if err == nil {
				break
			}
			select {
			case <-time.After(time.Second):
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout while waiting to send transaction: %v", t.TestName, err)
			}
		}

	}
	// Produce the next block with the fee recipient set
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	// Calculate the fees and check that they match the balance of the fee recipient
	blockIncluded, err := t.Eth.BlockByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%s): unable to get latest block: %v", t.TestName, err)
	}
	if len(blockIncluded.Transactions()) != txCount {
		t.Fatalf("FAIL (%s): not all transactions were included in block: %d != %d", t.TestName, len(blockIncluded.Transactions()), txCount)
	}
	if blockIncluded.Coinbase() != feeRecipient {
		t.Fatalf("FAIL (%s): feeRecipient was not set as coinbase: %v != %v", t.TestName, feeRecipient, blockIncluded.Coinbase())
	}
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

	feeRecipientBalance, err := t.Eth.BalanceAt(t.Ctx(), feeRecipient, nil)
	if feeRecipientBalance.Cmp(feeRecipientFees) != 0 {
		t.Fatalf("FAIL (%s): balance does not match fees: %v != %v", t.TestName, feeRecipientBalance, feeRecipientFees)
	}

	// Produce another block without txns and get the balance again
	t.CLMock.NextFeeRecipient = feeRecipient
	t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

	feeRecipientBalance, err = t.Eth.BalanceAt(t.Ctx(), feeRecipient, nil)
	if feeRecipientBalance.Cmp(feeRecipientFees) != 0 {
		t.Fatalf("FAIL (%s): balance does not match fees: %v != %v", t.TestName, feeRecipientBalance, feeRecipientFees)
	}
}

// TODO: Do a PENDING block suggestedFeeRecipient

func checkPrevRandaoValue(t *TestEnv, expectedPrevRandao common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	opcodeValueAtBlock, err := t.Eth.StorageAt(t.Ctx(), prevRandaoContractAddr, storageKey, nil)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get storage: %v", t.TestName, err)
	}
	if common.BytesToHash(opcodeValueAtBlock) != expectedPrevRandao {
		t.Fatalf("FAIL (%s): Storage does not match prevRandao: %v, %v", t.TestName, expectedPrevRandao, common.BytesToHash(opcodeValueAtBlock))
	}
}

// PrevRandao Opcode tests
func prevRandaoOpcodeTx(t *TestEnv) {
	// We need to send PREVRANDAO opcode transactions in PoW and particularly in the block where the TTD is reached.
	ttdReached := make(chan interface{})

	// Try to send many transactions before PoW transition to guarantee at least one enters in the block
	go func(t *TestEnv) {
		for {
			select {
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout while sending PREVRANDAO opcode transactions: %v")
			case <-ttdReached:
				return
			case <-time.After(time.Second / 10):
				tx := t.makeNextTransaction(prevRandaoContractAddr, big0, nil)
				if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
					t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
				}
				t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())
			}
		}
	}(t)
	t.CLMock.waitForTTD()
	close(ttdReached)

	// Ideally all blocks up until TTD must have a DIFFICULTY opcode tx in it
	ttdBlockNumber, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	// Start
	for i := uint64(ttdBlockNumber); i <= ttdBlockNumber; i++ {
		// First check that the block actually contained the transaction
		block, err := t.Eth.BlockByNumber(t.Ctx(), big.NewInt(int64(i)))
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to get block: %v", t.TestName, err)
		}
		if block.Transactions().Len() == 0 {
			t.Fatalf("FAIL (%s): (Test issue) no transactions went in block %d", t.TestName, i)
		}
		storageKey := common.Hash{}
		storageKey[31] = byte(i)
		opcodeValueAtBlock, err := getBigIntAtStorage(t.Eth, t.Ctx(), prevRandaoContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to get storage: %v", t.TestName, err)
		}
		if opcodeValueAtBlock.Cmp(big.NewInt(2)) != 0 {
			t.Fatalf("FAIL (%s): Incorrect difficulty value in block %d <=TTD: %v", t.TestName, i, opcodeValueAtBlock)
		}
	}

	// Send transactions now past TTD, the value of the storage in these blocks must match the prevRandao value
	var (
		txCount        = 10
		currentTxIndex = 0
		txs            = make([]*types.Transaction, 0)
	)
	t.CLMock.produceBlocks(txCount, BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			tx := t.makeNextTransaction(prevRandaoContractAddr, big0, nil)
			if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
				t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
			}
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
		c, ec, err := t.StartClient(client, newParams, t.MainTTD())
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

func exTransitionConfigPreMerge(t *TestEnv) {
	// Verify transition information
	t.VerifyTransitionInformation(t.Engine)
}

func exTransitionConfigPostMerge(t *TestEnv) {
	// Wait for TTD to happen
	t.CLMock.waitForTTD()
	// Verify transition information
	t.VerifyTransitionInformation(t.Engine)
}
