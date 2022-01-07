package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/catalyst"
	"github.com/ethereum/go-ethereum/params"
)

var (
	big0 = new(big.Int)
	big1 = big.NewInt(1)
)

// Engine API during PoW Negative tests: Client should reject Engine directives if the TTD has not been reached.
func engineAPIPoWTests(t *TestEnv) {
	// Test that the engine_ directives are correctly ignored when the chain has not yet reached TTD
	gblock := loadGenesis()
	if !t.CLMock.TTDReached {
		// We can only execute this test if we have not yet reached the TTD.
		forkchoiceState := catalyst.ForkchoiceStateV1{
			HeadBlockHash:      gblock.Hash(),
			SafeBlockHash:      gblock.Hash(),
			FinalizedBlockHash: gblock.Hash(),
		}
		resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceState, nil)
		if err == nil {
			t.Fatalf("FAIL (%v): ForkchoiceUpdated accepted under PoW rule: %v, %v", t.TestName, resp, err)
		}
		payloadresp, err := t.Engine.EngineGetPayloadV1(t.Engine.Ctx(), &hexutil.Bytes{1, 2, 3, 4, 5, 6, 7, 8})
		if err == nil {
			t.Fatalf("FAIL (%v): GetPayloadV1 accepted under PoW rule: %v, %v", t.TestName, payloadresp, err)
		}
		// Create a dummy payload to send in the ExecutePayload call
		payload := catalyst.ExecutableDataV1{
			ParentHash:    common.Hash{},
			FeeRecipient:  common.Address{},
			StateRoot:     common.Hash{},
			ReceiptsRoot:  common.Hash{},
			LogsBloom:     []byte{},
			Random:        common.Hash{},
			Number:        0,
			GasLimit:      0,
			GasUsed:       0,
			Timestamp:     0,
			ExtraData:     []byte{},
			BaseFeePerGas: big.NewInt(0),
			BlockHash:     common.Hash{},
			Transactions:  [][]byte{},
		}
		execresp, err := t.Engine.EngineExecutePayloadV1(t.Engine.Ctx(), &payload)
		if err == nil {
			t.Fatalf("FAIL (%v): ExecutePayloadV1 accepted under PoW rule: %v, %v", t.TestName, execresp, err)
		}
	}

}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using ExecutePayload) and unknown SafeBlock
// results in error
func unknownSafeBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}
	// Wait for ExecutePayload
	t.CLMock.NewExecutePayloadMutex.LockSet()
	defer t.CLMock.NewExecutePayloadMutex.Unlock()

	// Generate a random SafeBlock hash
	randomSafeBlockHash := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		randomSafeBlockHash[i] = byte(rand.Uint32())
	}

	// Send forkchoiceUpdated with random SafeBlockHash
	forkchoiceStateUnknownSafeHash := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		SafeBlockHash:      randomSafeBlockHash,
		FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
	}
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownSafeHash, nil)
	if err == nil {
		t.Fatalf("FAIL (%v): No error on forkchoiceUpdated with unknown SafeBlockHash: %v, %v", t.TestName, err, resp)
	}
}

// Verify that a forkchoiceUpdated with a valid HeadBlock (previously sent using ExecutePayload) and unknown
// FinalizedBlockHash results in error
func unknownFinalizedBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait for ExecutePayload
	t.CLMock.NewExecutePayloadMutex.LockSet()
	defer t.CLMock.NewExecutePayloadMutex.Unlock()

	// Generate a random FinalizedBlockHash hash
	randomFinalizedBlockHash := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		randomFinalizedBlockHash[i] = byte(rand.Uint32())
	}

	// Send forkchoiceUpdated with random FinalizedBlockHash
	forkchoiceStateUnknownFinalizedHash := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.LatestExecutedPayload.BlockHash,
		SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
		FinalizedBlockHash: randomFinalizedBlockHash,
	}
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
	}
	if resp.Status != "SYNCING" {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown FinalizedBlockHash is not SYNCING: %v, %v", t.TestName, resp)
	}

	// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
	payloadAttr := catalyst.PayloadAttributesV1{
		Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
		Random:                common.Hash{},
		SuggestedFeeRecipient: common.Address{},
	}
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownFinalizedHash, &payloadAttr)
	if err != nil {
		t.Fatalf("FAIL (%v): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
	}
	if resp.Status != "SYNCING" {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown FinalizedBlockHash is not SYNCING: %v, %v", t.TestName, resp)
	}
	if resp.PayloadID != nil {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown FinalizedBlockHash contains PayloadID: %v, %v", t.TestName, resp)
	}
}

// Verify that an unknown hash at HeadBlock in the forkchoice results in client returning "SYNCING" state
func unknownHeadBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait for FinalizedBlock
	t.CLMock.NewFinalizedBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewFinalizedBlockForkchoiceMutex.Unlock()

	// Generate a random HeadBlock hash
	randomHeadBlockHash := common.Hash{}
	for i := 0; i < common.HashLength; i++ {
		randomHeadBlockHash[i] = byte(rand.Uint32())
	}

	forkchoiceStateUnknownHeadHash := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      randomHeadBlockHash,
		SafeBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
		FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
	}

	fmt.Printf("INFO (unknownHeadBlockHash) forkchoiceStateUnknownHeadHash: %v\n", forkchoiceStateUnknownHeadHash)

	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownHeadHash, nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Error on forkchoiceUpdated with unknown HeadBlockHash: %v", t.TestName, err)
	}
	if resp.Status != "SYNCING" {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown HeadBlockHash is not SYNCING: %v", t.TestName, resp)
	}

	// Test again using PayloadAttributes, should also return SYNCING and no PayloadID
	payloadAttr := catalyst.PayloadAttributesV1{
		Timestamp:             t.CLMock.LatestExecutedPayload.Timestamp + 1,
		Random:                common.Hash{},
		SuggestedFeeRecipient: common.Address{},
	}
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateUnknownHeadHash, &payloadAttr)
	if err != nil {
		t.Fatalf("FAIL (%v): Error on forkchoiceUpdated with unknown HeadBlockHash: %v, %v", t.TestName, err)
	}
	if resp.Status != "SYNCING" {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown HeadBlockHash is not SYNCING: %v, %v", t.TestName, resp)
	}
	if resp.PayloadID != nil {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with unknown HeadBlockHash contains PayloadID: %v, %v", t.TestName, resp)
	}

}

// Verify that a forkchoiceUpdated fails on hash being set to a pre-TTD block after PoS change
func preTTDFinalizedBlockHash(t *TestEnv) {
	// Wait until TTD is reached by this client
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait for ExecutePayload
	t.CLMock.NewFinalizedBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewFinalizedBlockForkchoiceMutex.Unlock()

	// Send the Genesis block as forkchoice
	gblock := loadGenesis()
	forkchoiceStateGenesisHash := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      gblock.Hash(),
		SafeBlockHash:      gblock.Hash(),
		FinalizedBlockHash: gblock.Hash(),
	}
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &forkchoiceStateGenesisHash, nil)

	/* TBD: Behavior on this edge-case is undecided, as behavior of the Execution client
	 		if not defined on re-orgs to a point before the latest finalized block.

	if err == nil {
		t.Fatalf("FAIL (%v): No error forkchoiceUpdated with genesis: %v, %v", t.TestName, err, resp)
	}
	*/

	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Error on forkchoiceUpdated with unknown FinalizedBlockHash: %v, %v", t.TestName, err)
	}
	if resp.Status != "SUCCESS" {
		t.Fatalf("FAIL (%v): Response on forkchoiceUpdated with genesis is not SUCCESS: %v, %v", t.TestName, resp)
	}
}

func badHashOnExecPayload(t *TestEnv) {
	// Wait until TTD is reached by this client
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait for GetPayload
	t.CLMock.NewGetPayloadMutex.LockSet()
	defer t.CLMock.NewGetPayloadMutex.Unlock()

	// Alter hash on the payload and send it to client, should produce an error
	alteredPayload := t.CLMock.LatestPayloadBuilt
	alteredPayload.BlockHash[common.HashLength-1] = byte(255 - alteredPayload.BlockHash[common.HashLength-1])
	execPayloadResp, err := t.Engine.EngineExecutePayloadV1(t.Engine.Ctx(), &alteredPayload)
	if err == nil {
		t.Fatalf("FAIL (%v): Incorrect block hash in execute payload was not rejected: %v", t.TestName, execPayloadResp)
	}
}

// Test to verify Block information available at the Eth RPC after ExecutePayload
func blockStatusExecPayload(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Run executePayload tests
	t.CLMock.NewExecutePayloadMutex.LockSet()
	defer t.CLMock.NewExecutePayloadMutex.Unlock()
	latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.CLMock.NewHeadBlockForkchoiceMutex.Unlock()
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}
	// Latest block available via Eth RPC should not have changed at this point
	if latestBlockHeader.Hash() == t.CLMock.LatestExecutedPayload.BlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header incorrect after executePayload: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}

}

// Test to verify Block information available at the Eth RPC after new HeadBlock ForkchoiceUpdated
func blockStatusHeadBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Run HeadBlock tests
	t.CLMock.NewHeadBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewHeadBlockForkchoiceMutex.Unlock()
	latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}
	if latestBlockHeader.Hash() == t.CLMock.LatestForkchoice.HeadBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header doesn't match SafeBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}
}

// Test to verify Block information available at the Eth RPC after new SafeBlock ForkchoiceUpdated
func blockStatusSafeBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Run SafeBlock tests
	t.CLMock.NewSafeBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewSafeBlockForkchoiceMutex.Unlock()
	latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}
	if latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
		latestBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header doesn't match SafeBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}
}

// Test to verify Block information available at the Eth RPC after new FinalizedBlock ForkchoiceUpdated
func blockStatusFinalizedBlock(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Run FinalizedBlock tests
	t.CLMock.NewFinalizedBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewFinalizedBlockForkchoiceMutex.Unlock()
	latestBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}
	if latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
		latestBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header doesn't match FinalizedBlock hash: %v, %v", t.TestName, latestBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}
}

// Test to verify Block information available after a reorg using forkchoiceUpdated
func blockStatusReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS Block
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait until we reach SafeBlock status
	t.CLMock.NewSafeBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewSafeBlockForkchoiceMutex.Unlock()

	// Verify the client is serving the latest SafeBlock
	currentBlockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}
	if currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.HeadBlockHash ||
		currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.SafeBlockHash ||
		currentBlockHeader.Hash() == t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header doesn't match SafeBlock hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}

	// Reorg back to the previous block (FinalizedBlock)
	reorgForkchoice := catalyst.ForkchoiceStateV1{
		HeadBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
		SafeBlockHash:      t.CLMock.LatestForkchoice.FinalizedBlockHash,
		FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
	}
	resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &reorgForkchoice, nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
	}
	if resp.Status != "SUCCESS" {
		t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
	}

	// Check that we reorg to the previous block
	currentBlockHeader, err = t.Eth.HeaderByNumber(t.Ctx(), nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block header: %v", t.TestName, err)
	}

	if currentBlockHeader.Hash() != t.CLMock.LatestForkchoice.FinalizedBlockHash {
		t.Fatalf("FAIL (%v): latest block header doesn't match reorg hash: %v, %v", t.TestName, currentBlockHeader.Hash(), t.CLMock.LatestForkchoice)
	}

	// Send the SafeBlock again to leave everything back the way it was
	resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
	if err != nil {
		t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
	}
	if resp.Status != "SUCCESS" {
		t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
	}

}

// Test to re-org to a previous hash
func transactionReorg(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Create transactions that modify the state in order to check after the reorg.
	var (
		key                = t.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce              = uint64(0)
		txCount            = 5
		sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
	)
	var txs = make([]*types.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		data := make([]byte, 1)
		data[0] = byte(i)
		data = common.LeftPadBytes(data, 32)
		fmt.Printf("transactionReorg, i=%v, data=%v\n", i, data)
		rawTx := types.NewTransaction(nonce, sstoreContractAddr, big0, 100000, gasPrice, data)
		nonce++
		tx, err := t.Vault.signTransaction(key, rawTx)
		txs[i] = tx
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to sign deploy tx: %v", t.TestName, err)
		}
		if err = t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
			t.Fatalf("FAIL (%v): Unable to send transaction: %v", t.TestName, err)
		}
		time.Sleep(PoSBlockProductionPeriod)
	}
	var receipts = make([]*types.Receipt, txCount)
	for i := 0; i < txCount; i++ {
		receipt, err := waitForTxConfirmations(t, txs[i].Hash(), PoSConfirmationBlocks)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to fetch confirmed tx receipt: %v", t.TestName, err)
		}
		if receipt == nil {
			t.Fatalf("FAIL (%v): Unable to confirm tx: %v", t.TestName, txs[i].Hash())
		}
		receipts[i] = receipt
	}
	// Wait for a finalized block so we can start rolling back transactions
	t.CLMock.NewFinalizedBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewFinalizedBlockForkchoiceMutex.Unlock()

	for i := 0; i < txCount; i++ {

		data := make([]byte, 1)
		data[0] = byte(i)

		data = append(common.LeftPadBytes(data, 32), common.LeftPadBytes(key.Bytes(), 32)...)

		storageKey := crypto.Keccak256Hash(data)

		value_after, err := getBigIntAtStorage(t, sstoreContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%v): Could not get storage: %v", t.TestName, err)
		}
		fmt.Printf("transactionReorg, stor[%v]: %v\n", i, value_after)

		if value_after.Cmp(common.Big1) != 0 {
			t.Fatalf("FAIL (%v): Expected storage not set after transaction: %v", t.TestName, value_after)
		}

		// Get value at a block before the tx was included
		reorgBlock, err := t.Eth.BlockByNumber(t.Ctx(), receipts[i].BlockNumber.Sub(receipts[i].BlockNumber, common.Big1))
		value_before, err := getBigIntAtStorage(t, sstoreContractAddr, storageKey, reorgBlock.Number())
		if err != nil {
			t.Fatalf("FAIL (%v): Could not get storage: %v", t.TestName, err)
		}
		fmt.Printf("transactionReorg, stor[%v]: %v\n", i, value_before)

		if value_before.Cmp(common.Big0) != 0 {
			t.Fatalf("FAIL (%v): Expected storage not set after transaction: %v", t.TestName, value_before)
		}

		// Re-org back to a previous block where the tx is not included using forkchoiceUpdated
		reorgForkchoice := catalyst.ForkchoiceStateV1{
			HeadBlockHash:      reorgBlock.Hash(),
			SafeBlockHash:      reorgBlock.Hash(),
			FinalizedBlockHash: reorgBlock.Hash(),
		}
		resp, err := t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &reorgForkchoice, nil)
		if err != nil {
			t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
		if resp.Status != "SUCCESS" {
			t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}

		// Check storage again, should be unset
		value_before, err = getBigIntAtStorage(t, sstoreContractAddr, storageKey, nil)
		if err != nil {
			t.Fatalf("FAIL (%v): Could not get storage: %v", t.TestName, err)
		}
		fmt.Printf("transactionReorg, stor[%v]: %v\n", i, value_before)

		if value_before.Cmp(common.Big0) != 0 {
			t.Fatalf("FAIL (%v): Expected storage not set after transaction: %v", t.TestName, value_before)
		}

		// Re-send latest forkchoice to test next transaction
		resp, err = t.Engine.EngineForkchoiceUpdatedV1(t.Engine.Ctx(), &t.CLMock.LatestForkchoice, nil)
		if err != nil {
			t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
		if resp.Status != "SUCCESS" {
			t.Fatalf("FAIL (%v): Could not send forkchoiceUpdatedV1: %v", t.TestName, err)
		}
	}
}

// Re-Execute Previous Payloads
func reExecPayloads(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// Wait until we have the required number of payloads executed
	var payloadReExecCount = int64(10)
	_, err := waitForBlock(t, big.NewInt(t.CLMock.FirstPoSBlockNumber.Int64()+payloadReExecCount))
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to wait for %v executed payloads: %v", t.TestName, payloadReExecCount, err)
	}

	// Wait for a finalized block so we can re-executing payloads
	t.CLMock.NewFinalizedBlockForkchoiceMutex.LockSet()
	defer t.CLMock.NewFinalizedBlockForkchoiceMutex.Unlock()

	lastBlock, err := t.Eth.BlockNumber(t.Ctx())
	if err != nil {
		t.Fatalf("FAIL (%v): Unable to get latest block number: %v", t.TestName, err)
	}
	for i := lastBlock - uint64(payloadReExecCount); i <= lastBlock; i++ {
		payload := t.CLMock.ExecutedPayloadHistory[i]
		execPayloadResp, err := t.Engine.EngineExecutePayloadV1(t.Engine.Ctx(), &payload)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to re-execute valid payload: %v", err)
		}
		if execPayloadResp.Status != "VALID" {
			t.Fatalf("FAIL (%v): Unexpected status after re-execute valid payload: %v", execPayloadResp)
		}
	}
}

// Fee Recipient Tests
func suggestedFeeRecipient(t *TestEnv) {
	// Wait until this client catches up with latest PoS
	if !t.WaitForPoSSync() {
		t.Fatalf("FAIL (%v): Timeout on PoS sync", t.TestName)
	}

	// We need to verify at least
	// 	(a) one block without any fees
	// 	(b) one block with fees
	var noFeesBlock, feesBlock bool

	// Amount of transactions to send
	txCount := 10
	blockNumbers := make([]*big.Int, txCount)
	feeRecipients := make([]common.Address, txCount)

	for i := 0; i < txCount; i++ {
		// Set a random feeRecipient for `txCount` blocks.
		randAddr := make([]byte, 20)
		rand.Read(randAddr)
		feeRecipients[i] = common.BytesToAddress(randAddr)
		blockNumbers[i] = t.CLMock.setNextFeeRecipient(feeRecipients[i], t.Engine)
	}

	for i := 0; i < txCount; i++ {
		// Check each of the blocks and verify the fees were correctly assigned.
		feeRecipientAddress := feeRecipients[i]
		blockNumberIncluded := blockNumbers[i]
		if blockNumberIncluded == nil {
			t.Fatalf("FAIL (%v): unable to get block number included", t.TestName)
		}
		blockIncluded, err := waitForBlock(t, blockNumberIncluded)
		if err != nil {
			t.Fatalf("FAIL (%v): unable to get block with fee recipient: %v", t.TestName, err)
		}
		if blockIncluded.Coinbase() != feeRecipientAddress {
			t.Fatalf("FAIL (%v): Expected feeRecipient is not header.coinbase: %v!=%v", t.TestName, blockIncluded.Coinbase, feeRecipientAddress)
		}

		// Since the recipient address is random, we can assume the previous balance was zero.
		balanceAfter, err := t.Eth.BalanceAt(t.Ctx(), feeRecipientAddress, blockNumberIncluded)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to obtain balanceAfter: %v", t.TestName, err)
		}

		// We calculate the expected fees based on the transactions included.
		feeRecipientFee := big.NewInt(0)
		for _, tx := range blockIncluded.Transactions() {
			effGasTip, err := tx.EffectiveGasTip(blockIncluded.BaseFee())
			if err != nil {
				t.Fatalf("FAIL (%v): unable to obtain EffectiveGasTip: %v", t.TestName, err)
			}
			receipt, err := t.Eth.TransactionReceipt(t.Ctx(), tx.Hash())
			if err != nil {
				t.Fatalf("FAIL (%v): unable to obtain receipt: %v", t.TestName, err)
			}
			feeRecipientFee = feeRecipientFee.Add(feeRecipientFee, effGasTip.Mul(effGasTip, big.NewInt(int64(receipt.GasUsed))))
		}

		if feeRecipientFee.Cmp(balanceAfter) != 0 {
			t.Fatalf("FAIL (%v): actual fee received does not match feeRecipientFee: %v, %v", t.TestName, balanceAfter, feeRecipientFee)
		}
		if feeRecipientFee.Cmp(big0) == 0 {
			noFeesBlock = true
		} else {
			feesBlock = true
		}
	}

	if !(noFeesBlock && feesBlock) {
		if !noFeesBlock {
			t.Fatalf("FAIL (%v): Unable to test blocks without fees", t.TestName)
		} else {
			t.Fatalf("FAIL (%v): Unable to test blocks with fees", t.TestName)
		}
	}

}

// Random Opcode tests
func randomOpcodeTx(t *TestEnv) {
	var (
		key                = t.Vault.createAccount(t, big.NewInt(params.Ether))
		nonce              = uint64(0)
		txCount            = 20
		randomContractAddr = common.HexToAddress("0000000000000000000000000000000000000316")
	)
	var txs = make([]*types.Transaction, txCount)
	for i := 0; i < txCount; i++ {
		rawTx := types.NewTransaction(nonce, randomContractAddr, big0, 100000, gasPrice, nil)
		nonce++
		tx, err := t.Vault.signTransaction(key, rawTx)
		txs[i] = tx
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to sign deploy tx: %v", t.TestName, err)
		}
		if err = t.Eth.SendTransaction(t.Ctx(), tx); err != nil {
			bal, _ := t.Eth.BalanceAt(t.Ctx(), key, nil)
			fmt.Printf("Random: balance=%v\n", bal)
			t.Fatalf("FAIL (%v): Unable to send transaction: %v", t.TestName, err)
		}
		time.Sleep(PoSBlockProductionPeriod / 2)
	}
	PoWBlocks := 0
	PoSBlocks := 0
	i := 0
	for {
		receipt, err := waitForTxConfirmations(t, txs[i].Hash(), PoWConfirmationBlocks)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to fetch confirmed tx receipt: %v", t.TestName, err)
		}
		if receipt == nil {
			t.Fatalf("FAIL (%v): Unable to confirm tx: %v", t.TestName, txs[i].Hash())
		}

		blockHeader, err := t.Eth.HeaderByNumber(t.Ctx(), receipt.BlockNumber)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to fetch block header: %v", t.TestName, err)
		}

		stor, err := t.Eth.StorageAt(t.Ctx(), randomContractAddr, common.BigToHash(receipt.BlockNumber), receipt.BlockNumber)
		if err != nil {
			t.Fatalf("FAIL (%v): Unable to fetch storage: %v, block hash=%v", t.TestName, err, receipt.BlockHash)
		}
		storHash := common.BytesToHash(stor)

		if t.CLMock.isBlockPoS(receipt.BlockNumber) {
			PoSBlocks++
			if t.CLMock.RandomHistory[receipt.BlockNumber.Uint64()] != storHash {
				t.Fatalf("FAIL (%v): Storage does not match random: %v, %v", t.TestName, t.CLMock.RandomHistory[receipt.BlockNumber.Uint64()], storHash)
			}
			if blockHeader.Difficulty.Cmp(big.NewInt(0)) != 0 {
				t.Fatalf("FAIL (%v): PoS Block (%v) difficulty not set to zero: %v", t.TestName, receipt.BlockNumber, blockHeader.Difficulty)
			}
			if blockHeader.MixDigest != storHash {
				t.Fatalf("FAIL (%v): PoS Block (%v) mix digest does not match random: %v", t.TestName, blockHeader.MixDigest, storHash)
			}
		} else {
			PoWBlocks++
			if blockHeader.Difficulty.Cmp(storHash.Big()) != 0 {
				t.Fatalf("FAIL (%v): Storage does not match difficulty: %v, %v", t.TestName, blockHeader.Difficulty, storHash)
			}
			if blockHeader.Difficulty.Cmp(big.NewInt(0)) == 0 {
				t.Fatalf("FAIL (%v): PoW Block (%v) difficulty is set to zero: %v", t.TestName, receipt.BlockNumber, blockHeader.Difficulty)
			}
		}

		i++
		if i >= txCount {
			break
		}
	}
	if PoSBlocks == 0 {
		t.Fatalf("FAIL (%v): No Random Opcode transactions landed in PoS blocks", t.TestName)
	}
}
