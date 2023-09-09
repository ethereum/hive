package suite_engine

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"
)

// Test performing a re-org that involves removing or modifying a transaction
type TransactionReOrgTest struct {
	test.BaseSpec
	ReorgOut            bool
	ReorgDifferentBlock bool
	NewPayloadOnRevert  bool
}

func (s TransactionReOrgTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s TransactionReOrgTest) GetName() string {
	name := "Transaction Re-Org"
	if s.ReorgOut {
		name += ", Re-Org Out"
	}
	if s.ReorgDifferentBlock {
		name += ", Re-Org to Different Block"
	}
	if s.NewPayloadOnRevert {
		name += ", New Payload on Revert Back"
	}
	return name
}

// Test transaction status after a forkchoiceUpdated re-orgs to an alternative hash where a transaction is not present
func (spec TransactionReOrgTest) Execute(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Create transactions that modify the state in order to check after the reorg.
	var (
		txCount            = 5
		sstoreContractAddr = common.HexToAddress("0000000000000000000000000000000000000317")
	)

	for i := 0; i < txCount; i++ {
		var (
			altPayload *typ.ExecutableData
			tx         typ.Transaction
		)
		// Generate two payloads, one with the transaction and the other one without it
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadAttributesGenerated: func() {
				// At this point we have not broadcast the transaction.
				if spec.ReorgOut {
					// Any payload we get should not contain any
					payloadAttributes := t.CLMock.LatestPayloadAttributes
					rand.Read(payloadAttributes.Random[:])
					r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, &payloadAttributes, t.CLMock.LatestHeader.Time)
					r.ExpectNoError()
					if r.Response.PayloadID == nil {
						t.Fatalf("FAIL (%s): No payload ID returned by TestEngineForkchoiceUpdated", t.TestName)
					}
					g := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, &payloadAttributes)
					g.ExpectNoError()
					altPayload = &g.Payload

					if len(altPayload.Transactions) != 0 {
						t.Fatalf("FAIL (%s): Empty payload contains transactions: %v", t.TestName, altPayload)
					}
				}

				// At this point we can broadcast the transaction and it will be included in the next payload
				// Data is the key where a `1` will be stored
				data := common.LeftPadBytes([]byte{byte(i)}, 32)
				t.Logf("transactionReorg, i=%v, data=%v\n", i, data)
				var err error
				tx, err = t.SendNextTransaction(
					t.TestContext,
					t.Engine,
					&helper.BaseTransactionCreator{
						Recipient:  &sstoreContractAddr,
						Amount:     big0,
						Payload:    data,
						TxType:     t.TestTransactionType,
						GasLimit:   75000,
						ForkConfig: t.ForkConfig,
					},
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}

				// Get the receipt
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				receipt, _ := t.Eth.TransactionReceipt(ctx, tx.Hash())
				if receipt != nil {
					t.Fatalf("FAIL (%s): Receipt obtained before tx included in block: %v", t.TestName, receipt)
				}
			},
			OnGetPayload: func() {
				// Check that indeed the payload contains the transaction
				if !helper.TransactionInPayload(&t.CLMock.LatestPayloadBuilt, tx) {
					t.Fatalf("FAIL (%s): Payload built does not contain the transaction: %v", t.TestName, t.CLMock.LatestPayloadBuilt)
				}
				if spec.ReorgDifferentBlock {
					// Create side payload with different hash
					var err error
					customizer := &helper.CustomPayloadData{
						ExtraData: &([]byte{0x01}),
					}
					altPayload, err = customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
					if err != nil {
						t.Fatalf("Error creating reorg payload %v", err)
					}

					if altPayload.ParentHash != t.CLMock.LatestPayloadBuilt.ParentHash {
						t.Fatalf("FAIL (%s): Incorrect parent hash for payloads: %v != %v", t.TestName, altPayload.ParentHash, t.CLMock.LatestPayloadBuilt.ParentHash)
					}
					if altPayload.BlockHash == t.CLMock.LatestPayloadBuilt.BlockHash {
						t.Fatalf("FAIL (%s): Incorrect hash for payloads: %v == %v", t.TestName, altPayload.BlockHash, t.CLMock.LatestPayloadBuilt.BlockHash)
					}
				}
			},
			OnNewPayloadBroadcast: func() {
				// Get the receipt
				ctx, cancel := context.WithTimeout(t.TestContext, globals.RPCTimeout)
				defer cancel()
				receipt, _ := t.Eth.TransactionReceipt(ctx, tx.Hash())
				if receipt != nil {
					t.Fatalf("FAIL (%s): Receipt obtained before tx included in block (NewPayload): %v", t.TestName, receipt)
				}
			},
			OnForkchoiceBroadcast: func() {
				// Transaction is now in the head of the canonical chain, re-org and verify it's removed
				// Get the receipt
				txt := t.TestEngine.TestTransactionReceipt(tx.Hash())
				txt.ExpectBlockHash(t.CLMock.LatestForkchoice.HeadBlockHash)

				if altPayload.ParentHash != t.CLMock.LatestPayloadBuilt.ParentHash {
					t.Fatalf("FAIL (%s): Incorrect parent hash for payloads: %v != %v", t.TestName, altPayload.ParentHash, t.CLMock.LatestPayloadBuilt.ParentHash)
				}
				if altPayload.BlockHash == t.CLMock.LatestPayloadBuilt.BlockHash {
					t.Fatalf("FAIL (%s): Incorrect hash for payloads: %v == %v", t.TestName, altPayload.BlockHash, t.CLMock.LatestPayloadBuilt.BlockHash)
				}

				if altPayload == nil {
					t.Fatalf("FAIL (%s): No payload to re-org to", t.TestName)
				}
				r := t.TestEngine.TestEngineNewPayload(altPayload)
				r.ExpectStatus(test.Valid)
				r.ExpectLatestValidHash(&altPayload.BlockHash)

				s := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
					HeadBlockHash:      altPayload.BlockHash,
					SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
					FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
				}, nil, altPayload.Timestamp)
				s.ExpectPayloadStatus(test.Valid)

				p := t.TestEngine.TestHeaderByNumber(Head)
				p.ExpectHash(altPayload.BlockHash)

				txt = t.TestEngine.TestTransactionReceipt(tx.Hash())
				if spec.ReorgOut {
					if txt.Receipt != nil {
						receiptJson, _ := json.MarshalIndent(txt.Receipt, "", "  ")
						t.Fatalf("FAIL (%s): Receipt was obtained when the tx had been re-org'd out: %s", t.TestName, receiptJson)
					}
				} else if spec.ReorgDifferentBlock {
					txt.ExpectBlockHash(altPayload.BlockHash)
				}

				// Re-org back
				if spec.NewPayloadOnRevert {
					r = t.TestEngine.TestEngineNewPayload(&t.CLMock.LatestPayloadBuilt)
					r.ExpectStatus(test.Valid)
					r.ExpectLatestValidHash(&t.CLMock.LatestPayloadBuilt.BlockHash)
				}
				t.CLMock.BroadcastForkchoiceUpdated(&t.CLMock.LatestForkchoice, nil, 1)

				// Not it should be back with main payload
				txt = t.TestEngine.TestTransactionReceipt(tx.Hash())
				txt.ExpectBlockHash(t.CLMock.LatestForkchoice.HeadBlockHash)
			},
		})

	}

}

// Test that performing a re-org back into a previous block of the canonical chain does not produce errors and the chain
// is still capable of progressing.
type ReOrgBackToCanonicalTest struct {
	test.BaseSpec
	// Depth of the re-org to back in the canonical chain
	ReOrgDepth uint64
	// Number of transactions to send on each payload
	TransactionPerPayload uint64
	// Whether to execute a sidechain payload on the re-org
	ExecuteSidePayloadOnReOrg bool
}

func (s ReOrgBackToCanonicalTest) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (s ReOrgBackToCanonicalTest) GetName() string {
	name := fmt.Sprintf("Re-Org Back into Canonical Chain (Depth: %d)", s.ReOrgDepth)

	if s.ExecuteSidePayloadOnReOrg {
		name += ", Execute Side Payload on Re-Org"
	}
	return name
}

func (s ReOrgBackToCanonicalTest) GetDepth() uint64 {
	if s.ReOrgDepth == 0 {
		return 3
	}
	return s.ReOrgDepth
}

// Test that performing a re-org back into a previous block of the canonical chain does not produce errors and the chain
// is still capable of progressing.
func (spec ReOrgBackToCanonicalTest) Execute(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Check the CLMock configured safe and finalized
	if t.CLMock.SlotsToSafe.Cmp(new(big.Int).SetUint64(spec.ReOrgDepth)) <= 0 {
		t.Fatalf("FAIL (%s): [TEST ISSUE] CLMock configured slots to safe less than re-org depth: %v <= %v", t.TestName, t.CLMock.SlotsToSafe, spec.ReOrgDepth)
	}
	if t.CLMock.SlotsToFinalized.Cmp(new(big.Int).SetUint64(spec.ReOrgDepth)) <= 0 {
		t.Fatalf("FAIL (%s): [TEST ISSUE] CLMock configured slots to finalized less than re-org depth: %v <= %v", t.TestName, t.CLMock.SlotsToFinalized, spec.ReOrgDepth)
	}

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// We are going to reorg back to a previous hash several times
	previousHash := t.CLMock.LatestForkchoice.HeadBlockHash
	previousTimestamp := t.CLMock.LatestPayloadBuilt.Timestamp

	if spec.ExecuteSidePayloadOnReOrg {
		var (
			sidePayload                 *typ.ExecutableData
			sidePayloadParentForkchoice api.ForkchoiceStateV1
			sidePayloadParentTimestamp  uint64
		)
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadAttributesGenerated: func() {
				payloadAttributes := t.CLMock.LatestPayloadAttributes
				rand.Read(payloadAttributes.Random[:])
				r := t.TestEngine.TestEngineForkchoiceUpdated(&t.CLMock.LatestForkchoice, &payloadAttributes, t.CLMock.LatestHeader.Time)
				r.ExpectNoError()
				if r.Response.PayloadID == nil {
					t.Fatalf("FAIL (%s): No payload ID returned by TestEngineForkchoiceUpdated", t.TestName)
				}
				g := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, &payloadAttributes)
				g.ExpectNoError()
				sidePayload = &g.Payload
				sidePayloadParentForkchoice = t.CLMock.LatestForkchoice
				sidePayloadParentTimestamp = t.CLMock.LatestHeader.Time
			},
		})
		// Continue producing blocks until we reach the depth of the re-org
		t.CLMock.ProduceBlocks(int(spec.GetDepth()-1), clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// Send a transaction on each payload of the canonical chain
				var err error
				_, err = t.SendNextTransactions(
					t.TestContext,
					t.Engine,
					&helper.BaseTransactionCreator{
						Recipient:  &ZeroAddr,
						Amount:     big1,
						Payload:    nil,
						TxType:     t.TestTransactionType,
						GasLimit:   75000,
						ForkConfig: t.ForkConfig,
					},
					spec.TransactionPerPayload,
				)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transactions: %v", t.TestName, err)
				}
			},
		})
		// On the last block, before executing the next payload of the canonical chain,
		// re-org back to the parent of the side payload and execute the side payload first
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnGetPayload: func() {
				// We are about to execute the new payload of the canonical chain, re-org back to
				// the side payload
				f := t.TestEngine.TestEngineForkchoiceUpdated(&sidePayloadParentForkchoice, nil, sidePayloadParentTimestamp)
				f.ExpectPayloadStatus(test.Valid)
				f.ExpectLatestValidHash(&sidePayloadParentForkchoice.HeadBlockHash)
				// Execute the side payload
				n := t.TestEngine.TestEngineNewPayload(sidePayload)
				n.ExpectStatus(test.Valid)
				n.ExpectLatestValidHash(&sidePayload.BlockHash)
				// At this point the next canonical payload will be executed by the CL mock, so we can
				// continue producing blocks
			},
		})
	} else {
		t.CLMock.ProduceBlocks(int(spec.GetDepth()), clmock.BlockProcessCallbacks{
			OnForkchoiceBroadcast: func() {
				// Send a fcU with the HeadBlockHash pointing back to the previous block
				forkchoiceUpdatedBack := api.ForkchoiceStateV1{
					HeadBlockHash:      previousHash,
					SafeBlockHash:      previousHash,
					FinalizedBlockHash: previousHash,
				}

				// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
				r := t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceUpdatedBack, nil, previousTimestamp)
				r.ExpectNoError()
			},
		})
	}

	// Verify that the client is pointing to the latest payload sent
	r := t.TestEngine.TestHeaderByNumber(Head)
	r.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

}
