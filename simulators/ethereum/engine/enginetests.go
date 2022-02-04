package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// Invalid Terminal Block in ForkchoiceUpdated: Client must reject ForkchoiceUpdated directives if the referenced HeadBlockHash does not meet the TTD requirement.
func invalidTerminalBlockForkchoiceUpdated(t *TestEnv) {
	gblock := loadGenesis()

	forkchoiceState := beacon.ForkchoiceStateV1{
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
	payloadResp, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), &beacon.PayloadID{1, 2, 3, 4, 5, 6, 7, 8})
	if err == nil {
		t.Fatalf("FAIL (%s): GetPayloadV1 accepted under PoW rule: %v", t.TestName, payloadResp)
	}

}

// Invalid Terminal Block in NewPayload: Client must reject NewPayload directives if the referenced ParentHash does not meet the TTD requirement.
func invalidTerminalBlockNewPayload(t *TestEnv) {
	gblock := loadGenesis()

	// Create a dummy payload to send in the ExecutePayload call
	payload := beacon.ExecutableDataV1{
		ParentHash:    gblock.Hash(),
		FeeRecipient:  common.Address{},
		StateRoot:     gblock.Root(),
		ReceiptsRoot:  types.EmptyUncleHash,
		LogsBloom:     types.CreateBloom(types.Receipts{}).Bytes(),
		Random:        common.Hash{},
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

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using ExecutePayload) and unknown SafeBlock
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
			forkchoiceStateUnknownSafeHash := beacon.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
				SafeBlockHash:      randomSafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}
			// Execution specification:
			// - This value MUST be either equal to or an ancestor of headBlockHash
			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownSafeHash, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown SafeBlockHash: %v", t.TestName, err)
			}
			if resp.PayloadStatus.Status != "INVALID" {
				t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown SafeBlockHash is not INVALID: %v", t.TestName, resp)
			}
			if resp.PayloadID != nil {
				t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown SafeBlockHash contains PayloadID: %v, %v", t.TestName, resp)
			}
		},
	})

}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using ExecutePayload) and unknown
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
			forkchoiceStateUnknownFinalizedHash := beacon.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: randomFinalizedBlockHash,
			}
			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
			}
			if resp.PayloadStatus.Status != "INVALID" {
				t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown FinalizedBlockHash is not INVALID: %v, %v", t.TestName, resp)
			}

			// Test again using PayloadAttributes, should also return INVALID and no PayloadID
			payloadAttr := beacon.PayloadAttributesV1{
				Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
				Random:                common.Hash{},
				SuggestedFeeRecipient: common.Address{},
			}
			resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, &payloadAttr)
			if err != nil {
				t.Fatalf("FAIL (%s): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
			}
			if resp.PayloadStatus.Status != "INVALID" {
				t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown FinalizedBlockHash is not INVALID: %v, %v", t.TestName, resp)
			}
			if resp.PayloadID != nil {
				t.Fatalf("FAIL (%s): Response on forkchoiceUpdated with unknown FinalizedBlockHash contains PayloadID: %v, %v", t.TestName, resp)
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

	forkchoiceStateUnknownHeadHash := beacon.ForkchoiceStateV1{
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
	payloadAttr := beacon.PayloadAttributesV1{
		Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
		Random:                common.Hash{},
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
	gblock := loadGenesis()
	forkchoiceStateGenesisHash := beacon.ForkchoiceStateV1{
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

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		// Run test after the new payload has been obtained
		OnGetPayload: func() {
			// Alter hash on the payload and send it to client, should produce an error
			alteredPayload := t.CLMock.LatestPayloadBuilt
			alteredPayload.BlockHash[common.HashLength-1] = byte(255 - alteredPayload.BlockHash[common.HashLength-1])
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

// Generate test cases for each field of ExecutePayload, where the payload contains a single invalid field and a valid hash.
func invalidPayloadTestCaseGen(payloadField string) func(*TestEnv) {
	return func(t *TestEnv) {
		// Wait until TTD is reached by this client
		t.CLMock.waitForTTD()

		// Produce blocks before starting the test
		t.CLMock.produceBlocks(5, BlockProcessCallbacks{})

		t.CLMock.produceSingleBlock(BlockProcessCallbacks{
			// Run test after the new payload has been obtained
			OnGetPayload: func() {

				// Alter the payload while maintaining a valid hash and send it to the client, should produce an error
				basePayload := t.CLMock.LatestPayloadBuilt
				customPayloadMods := make(map[string]CustomPayloadData)
				customPayloadMods["ParentHash"] = CustomPayloadData{
					ParentHash: func() *common.Hash {
						modParentHash := basePayload.ParentHash
						modParentHash[common.HashLength-1] = byte(255 - modParentHash[common.HashLength-1])
						return &modParentHash
					}(),
				}
				customPayloadMods["StateRoot"] = CustomPayloadData{
					StateRoot: func() *common.Hash {
						modStateRoot := basePayload.StateRoot
						modStateRoot[common.HashLength-1] = byte(255 - modStateRoot[common.HashLength-1])
						return &modStateRoot
					}(),
				}

				customPayloadMods["ReceiptsRoot"] = CustomPayloadData{
					ReceiptsRoot: func() *common.Hash {
						modReceiptsRoot := basePayload.ReceiptsRoot
						modReceiptsRoot[common.HashLength-1] = byte(255 - modReceiptsRoot[common.HashLength-1])
						return &modReceiptsRoot
					}(),
				}
				customPayloadMods["Number"] = CustomPayloadData{
					Number: func() *uint64 {
						modNumber := basePayload.Number - 1
						return &modNumber
					}(),
				}
				customPayloadMods["GasLimit"] = CustomPayloadData{
					GasLimit: func() *uint64 {
						modGasLimit := basePayload.GasLimit * 2
						return &modGasLimit
					}(),
				}
				customPayloadMods["GasUsed"] = CustomPayloadData{
					GasUsed: func() *uint64 {
						modGasUsed := basePayload.GasUsed - 1
						return &modGasUsed
					}(),
				}
				customPayloadMods["Timestamp"] = CustomPayloadData{
					Timestamp: func() *uint64 {
						modTimestamp := basePayload.Timestamp - 1
						return &modTimestamp
					}(),
				}

				customPayloadMod := customPayloadMods[payloadField]

				t.Logf("INFO (%v) customizing payload using: %v\n", t.TestName, customPayloadMod)
				alteredPayload, err := customizePayload(&t.CLMock.LatestPayloadBuilt, &customPayloadMod)
				t.Logf("INFO (%v) latest real getPayload (not executed): hash=%v contents=%v\n", t.TestName, t.CLMock.LatestPayloadBuilt.BlockHash, t.CLMock.LatestPayloadBuilt)
				t.Logf("INFO (%v) customized payload: hash=%v contents=%v\n", t.TestName, alteredPayload.BlockHash, alteredPayload)
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to modify payload (%v): %v", t.TestName, customPayloadMod, err)
				}
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
				fcState := beacon.ForkchoiceStateV1{
					HeadBlockHash:      alteredPayload.BlockHash,
					SafeBlockHash:      alteredPayload.BlockHash,
					FinalizedBlockHash: alteredPayload.BlockHash,
				}
				payloadAttrbutes := beacon.PayloadAttributesV1{
					Timestamp:             alteredPayload.Timestamp + 1,
					Random:                common.Hash{},
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
		// Run test after a forkchoice with new HeadBlockHash has been broadcasted
		OnHeadBlockForkchoiceBroadcast: func() {

			// Verify the client is serving the latest HeadBlock
			currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}
			if currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
				currentBlockHeader.Hash() == t.CLMock.LatestForkchoice.SafeBlockHash ||
				currentBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match HeadBlock hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice)
			}

			// Reorg back to the previous block (FinalizedBlock)
			reorgForkchoice := beacon.ForkchoiceStateV1{
				HeadBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}
			resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &reorgForkchoice, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
			}
			if resp.PayloadStatus.Status != "VALID" {
				t.Fatalf("FAIL (%s): Incorrect status returned after a HeadBlockHash reorg: %v", t.TestName, resp.PayloadStatus.Status)
			}

			if *resp.PayloadStatus.LatestValidHash != t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): Incorrect latestValidHash returned after a HeadBlockHash reorg: %v!=%v", t.TestName, resp.PayloadStatus.LatestValidHash, t.CLMock.LatestForkchoice.FinalizedBlockHash)
			}

			// Check that we reorg to the previous block
			currentBlockHeader, err = t.Eth.HeaderByNumber(t.Ctx(), nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to get latest block header: %v", t.TestName, err)
			}

			if currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
				t.Fatalf("FAIL (%s): latest block header doesn't match reorg hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice.FinalizedBlockHash)
			}

			// Send the HeadBlock again to leave everything back the way it was
			resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
			}
			if resp.PayloadStatus.Status != "VALID" {
				t.Fatalf("FAIL (%s): Incorrect status returned after a HeadBlockHash reorg: %v", t.TestName, resp.PayloadStatus.Status)
			}

			if *resp.PayloadStatus.LatestValidHash != t.CLMock.LatestForkchoice.HeadBlockHash {
				t.Fatalf("FAIL (%s): Incorrect latestValidHash returned after a HeadBlockHash reorg: %v!=%v", t.TestName, resp.PayloadStatus.LatestValidHash, t.CLMock.LatestForkchoice.FinalizedBlockHash)
			}

		},
	})

}

// Test transaction status after a forkchoiceUpdated re-orgs to previous hash
func transactionReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	t.CLMock.waitForTTD()

	// Create transactions that modify the state in order to check after the reorg.
	var (
		txCount            = 5
		sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
		receipts           = make([]*types.Receipt, txCount)
	)

	var txs = make([]*types.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		// Data is the key where a `1` will be stored
		data := common.LeftPadBytes([]byte{byte(i)}, 32)
		t.Logf("transactionReorg, i=%v, data=%v\n", i, data)
		tx := t.makeNextTransaction(sstoreContractAddr, big0, data)
		txs[i] = tx

		// Send the transaction
		if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
			t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
		}
		// Produce the block containing the transaction
		t.CLMock.produceSingleBlock(BlockProcessCallbacks{})

		// Get the receipt
		receipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to obtain transaction receipt: %v", t.TestName, err)
		}
		receipts[i] = receipt
	}

	for i := 0; i < txCount; i++ {
		// The sstore contract stores a `1` to key specified in data
		storageKey := common.Hash{}
		storageKey[31] = byte(i)

		valueWithTxApplied, err := getBigIntAtStorage(t.Eth, t.Ctx(), sstoreContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Could not get storage: %v", t.TestName, err)
		}
		t.Logf("transactionReorg, stor[%v]: %v\n", i, valueWithTxApplied)

		if valueWithTxApplied.Cmp(common.Big1) != 0 {
			t.Fatalf("FAIL (%s): Expected storage not set after transaction: %v", t.TestName, valueWithTxApplied)
		}

		// Get value at a block before the tx was included
		reorgBlock, err := t.Eth.BlockByNumber(t.Ctx(), receipts[i].BlockNumber.Sub(receipts[i].BlockNumber, common.Big1))
		valueWithoutTxApplied, err := getBigIntAtStorage(t.Eth, t.Ctx(), sstoreContractAddr, storageKey, reorgBlock.Number())
		if err != nil {
			t.Fatalf("FAIL (%s): Could not get storage: %v", t.TestName, err)
		}
		t.Logf("transactionReorg, stor[%v]: %v\n", i, valueWithoutTxApplied)

		if valueWithoutTxApplied.Cmp(common.Big0) != 0 {
			t.Fatalf("FAIL (%s): Expected storage not set after transaction: %v", t.TestName, valueWithoutTxApplied)
		}

		// Re-org back to a previous block where the tx is not included using forkchoiceUpdated
		reorgForkchoice := beacon.ForkchoiceStateV1{
			HeadBlockHash:      reorgBlock.Hash(),
			SafeBlockHash:      reorgBlock.Hash(),
			FinalizedBlockHash: reorgBlock.Hash(),
		}
		resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &reorgForkchoice, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
		if resp.PayloadStatus.Status != "VALID" {
			t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}

		// Check storage again using `latest`, should be unset
		valueWithoutTxApplied, err = getBigIntAtStorage(t.Eth, t.Ctx(), sstoreContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Could not get storage: %v", t.TestName, err)
		}
		t.Logf("transactionReorg, stor[%v]: %v\n", i, valueWithoutTxApplied)

		if valueWithoutTxApplied.Cmp(common.Big0) != 0 {
			t.Fatalf("FAIL (%s): Expected storage not set after transaction: %v", t.TestName, valueWithoutTxApplied)
		}

		// Re-send latest forkchoice to test next transaction
		resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
		if resp.PayloadStatus.Status != "VALID" {
			t.Fatalf("FAIL (%s): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
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
	tx := t.makeNextTransaction(randomContractAddr, big0, nil)
	if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
		t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.produceSingleBlock(BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativeRandom := common.Hash{}
			rand.Read(alternativeRandom[:])

			payloadAttributes := beacon.PayloadAttributesV1{
				Timestamp:             t.CLMock.LatestFinalizedHeader.Time + 1,
				Random:                alternativeRandom,
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
				t.Fatalf("FAIL (%s): alternative payload does not contain the random opcode tx", t.TestName)
			}
			alternativePayloadStatus, err := t.Engine.EngineNewPayloadV1(t.Engine.Ctx(), &alternativePayload)
			if err != nil {
				t.Fatalf("FAIL (%s): Could not send alternative payload: %v", t.TestName, err)
			}
			if alternativePayloadStatus.Status != "VALID" {
				t.Fatalf("FAIL (%s): Alternative payload response returned Status!=VALID: %v", t.TestName, alternativePayloadStatus)
			}
			// We sent the alternative payload, fcU to it
			alternativeFcU := beacon.ForkchoiceStateV1{
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

			// Random should be the alternative random we sent
			checkRandomValue(t, alternativeRandom, alternativePayload.Number)
		},
	})
	// The reorg actually happens after the CLMocker continues,
	// verify here that the reorg was successful
	latestBlockNum := t.CLMock.LatestFinalizedNumber.Uint64()
	checkRandomValue(t, t.CLMock.RandomHistory[latestBlockNum], latestBlockNum)

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

			// Fabricate and send multiple new payloads by changing the Random field
			for i := 0; i < payloadCount; i++ {
				newRandom := common.Hash{}
				rand.Read(newRandom[:])
				newPayload, err := customizePayload(&basePayload, &CustomPayloadData{
					Random: &newRandom,
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
	payloadCount := 10

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
	for _, client := range allClients {
		_, ec, err := t.StartClient(client, t.ClientParams)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to start client (%v): %v", t.TestName, client, err)
		}
		// Send the forkchoiceUpdated with the LatestExecutedPayload hash, we should get SYNCING back
		fcU := beacon.ForkchoiceStateV1{
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
				if payloadResp.Status != "ACCEPTED" {
					t.Fatalf("FAIL (%s): Incorrect Status!=ACCEPTED (Payload Number=%v): %v", t.TestName, i, payloadResp.Status)
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
		// Empty self tx
		tx := t.makeNextTransaction(vaultAccountAddr, big0, nil)
		if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
			t.Fatalf("FAIL (%s): unable to send transaction: %v", t.TestName, err)
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

func checkRandomValue(t *TestEnv, expectedRandom common.Hash, blockNumber uint64) {
	storageKey := common.Hash{}
	storageKey[31] = byte(blockNumber)
	opcodeValueAtBlock, err := t.Eth.StorageAt(t.Ctx(), randomContractAddr, storageKey, nil)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get storage: %v", t.TestName, err)
	}
	if common.BytesToHash(opcodeValueAtBlock) != expectedRandom {
		t.Fatalf("FAIL (%s): Storage does not match random: %v, %v", t.TestName, expectedRandom, common.BytesToHash(opcodeValueAtBlock))
	}
}

// Random Opcode tests
func randomOpcodeTx(t *TestEnv) {
	// We need to send random opcode transactions in PoW and particularly in the block where the TTD is reached.
	ttdReached := make(chan interface{})

	// Try to send many transactions before PoW transition to guarantee at least one enters in the block
	go func(t *TestEnv) {
		for {
			select {
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Timeout while sending random opcode transactions: %v")
			case <-ttdReached:
				return
			case <-time.After(time.Second / 10):
				tx := t.makeNextTransaction(randomContractAddr, big0, nil)
				if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
					t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
				}
				t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())
			}
		}
	}(t)
	t.CLMock.waitForTTD()
	close(ttdReached)

	// Ideally all blocks up until TTD must have a random tx in it
	ttdBlockNumber, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	// Start
	for i := uint64(2); i <= ttdBlockNumber; i++ {
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
		opcodeValueAtBlock, err := getBigIntAtStorage(t.Eth, t.Ctx(), randomContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to get storage: %v", t.TestName, err)
		}
		if opcodeValueAtBlock.Cmp(big.NewInt(2)) != 0 {
			t.Fatalf("FAIL (%s): Incorrect difficulty value in block %d <=TTD: %v", t.TestName, i, opcodeValueAtBlock)
		}
	}

	// Send transactions now past TTD, the value of the storage in these blocks must match the random value
	t.CLMock.produceBlocks(10, BlockProcessCallbacks{
		OnGetPayloadID: func() {
			tx := t.makeNextTransaction(randomContractAddr, big0, nil)
			if err := t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
				t.Fatalf("FAIL (%s): Unable to send transaction: %v", t.TestName, err)
			}
		},
	})

	lastBlockNumber, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to get latest block number: %v", t.TestName, err)
	}
	for i := ttdBlockNumber + 1; i <= lastBlockNumber; i++ {
		checkRandomValue(t, t.CLMock.RandomHistory[i], i)
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
	newParams := clientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", t.CLMock.TerminalTotalDifficulty.Int64()))
	newParams = newParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))
	newParams = newParams.Set("HIVE_MINER", "")

	for _, client := range allClients {
		c, ec, err := t.StartClient(client, newParams)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to start client (%v): %v", t.TestName, client, err)
		}
		// Add engine client and broadcast to it the latest forkchoice
		t.CLMock.AddEngineClient(t.T, c)
		t.CLMock.broadcastLatestForkchoice()
	syncLoop:
		for {
			select {
			case <-time.After(time.Second):
				bn, err := ec.Eth.BlockNumber(t.Ctx())
				if err != nil {
					t.Fatalf("FAIL (%s): Unable to obtain latest block", t.TestName)
				}
				if t.CLMock.LatestFinalizedNumber != nil && bn >= t.CLMock.LatestFinalizedNumber.Uint64() {
					t.Logf("INFO (%v): Client (%v) is now synced to latest PoS block", t.TestName, c.Container)
					break syncLoop
				}
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Test timeout", t.TestName)
			}

		}
	}
}

/*
func mismatchedTTDClientSync(t *TestEnv) {
	// Launch another client with a higher TTD configuration,
	// all executed payloads in the main client should be properly rejected.
	allClients, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain all client types", t.TestName)
	}
	// Set the new TTD + Bootnode
	newParams := clientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", t.CLMock.TerminalTotalDifficulty.Add(t.CLMock.TerminalTotalDifficulty, big.NewInt(2)).Uint64()))
	enode, err := t.Engine.EnodeURL()
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to obtain bootnode", err)
	}
	newParams = newParams.Set("HIVE_BOOTNODE", fmt.Sprintf("%s", enode))

	for _, client := range allClients {
		c2 := t.StartClient(client, newParams, files)
		ec := NewEngineClient(t.T, c2)
		for i := 0; i < 10; i++ {
			// Try running 10 payloads, all based on an chain with an apparent invalid TTD
			select {
			case <-t.CLMock.OnGetPayload:
				t.CLMock.OnGetPayload.Yield()
			case <-t.CLMock.OnExit:
				t.Fatalf("FAIL (%s): CLMocker stopped producing blocks", t.TestName)
			case <-t.Timeout:
				t.Fatalf("FAIL (%s): Test timeout", t.TestName)
			}
		}

	}
}
*/
