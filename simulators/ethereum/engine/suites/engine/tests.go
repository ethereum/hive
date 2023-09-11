package suite_engine

import (
	"encoding/json"
	"math/big"
	"math/rand"
	"time"

	api "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
	typ "github.com/ethereum/hive/simulators/ethereum/engine/types"

	"github.com/ethereum/go-ethereum/common"
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var (
	big0      = new(big.Int)
	big1      = big.NewInt(1)
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
	ZeroAddr  = common.Address{}
)

var Tests = []test.Spec{

	&test.BaseSpec{
		Name: "latest Block after Reorg",
		Run:  blockStatusReorg,
	},

	// Re-org using Engine API

	&test.BaseSpec{
		Name: "Sidechain Reorg",
		Run:  sidechainReorg,
	},
	&test.BaseSpec{
		Name: "Re-Org Back to Canonical Chain From Syncing Chain",
		Run:  reorgBackFromSyncing,
	},
	&test.BaseSpec{
		Name:             "Import and re-org to previously validated payload on a side chain",
		SlotsToSafe:      big.NewInt(15),
		SlotsToFinalized: big.NewInt(20),
		Run:              reorgPrevValidatedPayloadOnSideChain,
	},
	&test.BaseSpec{
		Name:             "Safe Re-Org to Side Chain",
		Run:              safeReorgToSideChain,
		SlotsToSafe:      big.NewInt(1),
		SlotsToFinalized: big.NewInt(2),
	},
}

// Test to verify Block information available after a reorg using forkchoiceUpdated
func blockStatusReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS Block
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Run using an alternative Payload, verify that the latest info is updated after re-org
			customRandom := common.Hash{}
			rand.Read(customRandom[:])
			customizer := &helper.CustomPayloadData{
				PrevRandao: &customRandom,
			}
			customizedPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}

			// Send custom payload and fcU to it
			t.CLMock.BroadcastNewPayload(customizedPayload, t.ForkConfig.NewPayloadVersion(customizedPayload.Timestamp))
			t.CLMock.BroadcastForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      customizedPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil, t.ForkConfig.NewPayloadVersion(customizedPayload.Timestamp))

			// Verify the client is serving the latest HeadBlock
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(customizedPayload.BlockHash)

		},
		OnForkchoiceBroadcast: func() {
			// At this point, we have re-org'd to the payload that the CLMocker was originally planning to send,
			// verify that the client is serving the latest HeadBlock.
			r := t.TestEngine.TestHeaderByNumber(Head)
			r.ExpectHash(t.CLMock.LatestForkchoice.HeadBlockHash)

		},
	})

}

// Test that performs a re-org to a previously validated payload on a side chain.
func reorgPrevValidatedPayloadOnSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	var (
		sidechainPayloads     = make([]*typ.ExecutableData, 0)
		sidechainPayloadCount = 5
	)

	// Produce a canonical chain while at the same time generate a side chain to which we will re-org.
	t.CLMock.ProduceBlocks(sidechainPayloadCount, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// The side chain will consist simply of the same payloads with extra data appended
			extraData := []byte("side")
			customData := helper.CustomPayloadData{
				ExtraData: &extraData,
			}
			if len(sidechainPayloads) > 0 {
				customData.ParentHash = &sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			altPayload, err := customData.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)

			r := t.TestEngine.TestEngineNewPayload(altPayload)
			r.ExpectStatus(test.Valid)
			r.ExpectLatestValidHash(&altPayload.BlockHash)
		},
	})

	// Attempt to re-org to one of the sidechain payloads, but not the leaf,
	// and also build a new payload from this sidechain.
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			var (
				prevRandao            = common.Hash{}
				suggestedFeeRecipient = common.Address{0x12, 0x34}
			)
			rand.Read(prevRandao[:])
			payloadAttributesCustomizer := &helper.BasePayloadAttributesCustomizer{
				Random:                &prevRandao,
				SuggestedFeeRecipient: &suggestedFeeRecipient,
			}

			reOrgPayload := sidechainPayloads[len(sidechainPayloads)-2]
			reOrgPayloadAttributes := sidechainPayloads[len(sidechainPayloads)-1].PayloadAttributes

			payloadAttributesJson, _ := json.MarshalIndent(reOrgPayloadAttributes, "", "  ")
			t.Logf("DEBUG: reOrgPayloadAttributes: %v\n", string(payloadAttributesJson))

			newPayloadAttributes, err := payloadAttributesCustomizer.GetPayloadAttributes(&reOrgPayloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}

			r := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      reOrgPayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, newPayloadAttributes, reOrgPayload.Timestamp)
			r.ExpectPayloadStatus(test.Valid)
			r.ExpectLatestValidHash(&reOrgPayload.BlockHash)

			p := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, newPayloadAttributes)
			p.ExpectPayloadParentHash(reOrgPayload.BlockHash)

			s := t.TestEngine.TestEngineNewPayload(&p.Payload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&p.Payload.BlockHash)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Test that performs a re-org of the safe block to a side chain.
func safeReorgToSideChain(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*typ.ExecutableData, 0)

	// Produce three payloads `P1`, `P2`, `P3`, along with the side chain payloads `P2'`, `P3'`
	// First payload is finalized so no alternative payload
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{})
	t.CLMock.ProduceBlocks(2, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			customizer := &helper.CustomPayloadData{
				ParentHash: &altParentHash,
				ExtraData:  &([]byte{0x01}),
			}
			altPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Verify current state of labels
	head := t.TestEngine.TestHeaderByNumber(Head)
	head.ExpectHash(t.CLMock.LatestPayloadBuilt.BlockHash)

	safe := t.TestEngine.TestHeaderByNumber(Safe)
	safe.ExpectHash(t.CLMock.ExecutedPayloadHistory[2].BlockHash)

	finalized := t.TestEngine.TestHeaderByNumber(Finalized)
	finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

	// Re-org the safe/head blocks to point to the alternative side chain
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			for _, p := range sidechainPayloads {
				r := t.TestEngine.TestEngineNewPayload(p)
				r.ExpectStatusEither(test.Valid, test.Accepted)
			}
			r := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[1].BlockHash,
				SafeBlockHash:      sidechainPayloads[0].BlockHash,
				FinalizedBlockHash: t.CLMock.ExecutedPayloadHistory[1].BlockHash,
			}, nil, sidechainPayloads[1].Timestamp)
			r.ExpectPayloadStatus(test.Valid)

			head := t.TestEngine.TestHeaderByNumber(Head)
			head.ExpectHash(sidechainPayloads[1].BlockHash)

			safe := t.TestEngine.TestHeaderByNumber(Safe)
			safe.ExpectHash(sidechainPayloads[0].BlockHash)

			finalized := t.TestEngine.TestHeaderByNumber(Finalized)
			finalized.ExpectHash(t.CLMock.ExecutedPayloadHistory[1].BlockHash)

		},
	})
}

// Test that performs a re-org back to the canonical chain after re-org to syncing/unavailable chain.
func reorgBackFromSyncing(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce an alternative chain
	sidechainPayloads := make([]*typ.ExecutableData, 0)
	t.CLMock.ProduceBlocks(10, clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			// Generate an alternative payload by simply adding extraData to the block
			altParentHash := t.CLMock.LatestPayloadBuilt.ParentHash
			if len(sidechainPayloads) > 0 {
				altParentHash = sidechainPayloads[len(sidechainPayloads)-1].BlockHash
			}
			customizer := &helper.CustomPayloadData{
				ParentHash: &altParentHash,
				ExtraData:  &([]byte{0x01}),
			}
			altPayload, err := customizer.CustomizePayload(&t.CLMock.LatestPayloadBuilt)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload: %v", t.TestName, err)
			}
			sidechainPayloads = append(sidechainPayloads, altPayload)
		},
	})

	// Produce blocks before starting the test (So we don't try to reorg back to the genesis block)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			r := t.TestEngine.TestEngineNewPayload(sidechainPayloads[len(sidechainPayloads)-1])
			r.ExpectStatusEither(test.Syncing, test.Accepted)
			r.ExpectLatestValidHash(nil)
			// We are going to send one of the alternative payloads and fcU to it
			forkchoiceUpdatedBack := api.ForkchoiceStateV1{
				HeadBlockHash:      sidechainPayloads[len(sidechainPayloads)-1].BlockHash,
				SafeBlockHash:      sidechainPayloads[len(sidechainPayloads)-2].BlockHash,
				FinalizedBlockHash: sidechainPayloads[len(sidechainPayloads)-3].BlockHash,
			}

			// It is only expected that the client does not produce an error and the CL Mocker is able to progress after the re-org
			s := t.TestEngine.TestEngineForkchoiceUpdated(&forkchoiceUpdatedBack, nil, sidechainPayloads[len(sidechainPayloads)-1].Timestamp)
			s.ExpectLatestValidHash(nil)
			s.ExpectPayloadStatus(test.Syncing)

			// After this, the CLMocker will continue and try to re-org to canonical chain once again
			// CLMocker will fail the test if this is not possible, so nothing left to do.
		},
	})
}

// Reorg to a Sidechain using ForkchoiceUpdated
func sidechainReorg(t *test.Env) {
	// Wait until this client catches up with latest PoS
	t.CLMock.WaitForTTD()

	// Produce blocks before starting the test
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})

	// Produce two payloads, send fcU with first payload, check transaction outcome, then reorg, check transaction outcome again

	// This single transaction will change its outcome based on the payload
	tx, err := t.SendNextTransaction(
		t.TestContext,
		t.Engine,
		&helper.BaseTransactionCreator{
			Recipient:  &globals.PrevRandaoContractAddr,
			Amount:     big0,
			Payload:    nil,
			TxType:     t.TestTransactionType,
			GasLimit:   75000,
			ForkConfig: t.ForkConfig,
		},
	)
	if err != nil {
		t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
	}
	t.Logf("INFO (%s): sent tx %v", t.TestName, tx.Hash())

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnNewPayloadBroadcast: func() {
			// At this point the CLMocker has a payload that will result in a specific outcome,
			// we can produce an alternative payload, send it, fcU to it, and verify the changes
			alternativePrevRandao := common.Hash{}
			rand.Read(alternativePrevRandao[:])
			timestamp := t.CLMock.LatestPayloadBuilt.Timestamp + 1
			payloadAttributes, err := (&helper.BasePayloadAttributesCustomizer{
				Timestamp: &timestamp,
				Random:    &alternativePrevRandao,
			}).GetPayloadAttributes(&t.CLMock.LatestPayloadAttributes)
			if err != nil {
				t.Fatalf("FAIL (%s): Unable to customize payload attributes: %v", t.TestName, err)
			}

			r := t.TestEngine.TestEngineForkchoiceUpdated(
				&t.CLMock.LatestForkchoice,
				payloadAttributes,
				t.CLMock.LatestPayloadBuilt.Timestamp,
			)
			r.ExpectNoError()

			time.Sleep(t.CLMock.PayloadProductionClientDelay)

			g := t.TestEngine.TestEngineGetPayload(r.Response.PayloadID, payloadAttributes)
			g.ExpectNoError()
			alternativePayload := g.Payload
			if len(alternativePayload.Transactions) == 0 {
				t.Fatalf("FAIL (%s): alternative payload does not contain the prevRandao opcode tx", t.TestName)
			}

			s := t.TestEngine.TestEngineNewPayload(&alternativePayload)
			s.ExpectStatus(test.Valid)
			s.ExpectLatestValidHash(&alternativePayload.BlockHash)

			// We sent the alternative payload, fcU to it
			p := t.TestEngine.TestEngineForkchoiceUpdated(&api.ForkchoiceStateV1{
				HeadBlockHash:      alternativePayload.BlockHash,
				SafeBlockHash:      t.CLMock.LatestForkchoice.SafeBlockHash,
				FinalizedBlockHash: t.CLMock.LatestForkchoice.FinalizedBlockHash,
			}, nil, alternativePayload.Timestamp)
			p.ExpectPayloadStatus(test.Valid)

			// PrevRandao should be the alternative prevRandao we sent
			checkPrevRandaoValue(t, alternativePrevRandao, alternativePayload.Number)
		},
	})
	// The reorg actually happens after the CLMocker continues,
	// verify here that the reorg was successful
	latestBlockNum := t.CLMock.LatestHeadNumber.Uint64()
	checkPrevRandaoValue(t, t.CLMock.PrevRandaoHistory[latestBlockNum], latestBlockNum)

}

// Engine API errors
func pUint64(v uint64) *uint64 {
	return &v
}

// Register all test combinations for Paris
func init() {
	// Register bad hash tests
	for _, syncing := range []bool{false, true} {
		for _, sidechain := range []bool{false, true} {
			Tests = append(Tests, BadHashOnNewPayload{
				Syncing:   syncing,
				Sidechain: sidechain,
			})
		}
	}

	// Parent hash == block hash tests
	Tests = append(Tests,
		ParentHashOnNewPayload{
			Syncing: false,
		},
		ParentHashOnNewPayload{
			Syncing: true,
		},
	)

	// Register RPC tests
	for _, field := range []BlockStatusRPCCheckType{
		LatestOnNewPayload,
		LatestOnHeadBlockHash,
		SafeOnSafeBlockHash,
		FinalizedOnFinalizedBlockHash,
	} {
		Tests = append(Tests, BlockStatus{CheckType: field})
	}

	// Register ForkchoiceUpdate tests
	for _, field := range []ForkchoiceStateField{
		HeadBlockHash,
		SafeBlockHash,
		FinalizedBlockHash,
	} {
		Tests = append(Tests,
			InconsistentForkchoiceTest{
				Field: field,
			},
			ForkchoiceUpdatedUnknownBlockHashTest{
				Field: field,
			},
		)
	}

	// Payload Attributes Tests
	for _, t := range []InvalidPayloadAttributesTest{
		{
			Description: "Zero timestamp",
			Customizer: &helper.BasePayloadAttributesCustomizer{
				Timestamp: pUint64(0),
			},
		},
		{
			Description: "Parent timestamp",
			Customizer: &helper.TimestampDeltaPayloadAttributesCustomizer{
				PayloadAttributesCustomizer: &helper.BasePayloadAttributesCustomizer{},
				TimestampDelta:              -1,
			},
		},
	} {
		Tests = append(Tests, t)
		t.Syncing = true
		Tests = append(Tests, t)
	}

	// Payload ID Tests
	for _, payloadAttributeFieldChange := range []PayloadAttributesFieldChange{
		PayloadAttributesIncreaseTimestamp,
		PayloadAttributesRandom,
		PayloadAttributesSuggestedFeeRecipient,
	} {
		Tests = append(Tests, UniquePayloadIDTest{
			FieldModification: payloadAttributeFieldChange,
		})
	}

	// Endpoint Versions Tests
	// Early upgrade of ForkchoiceUpdated when requesting a payload
	Tests = append(Tests,
		ForkchoiceUpdatedOnPayloadRequestTest{
			BaseSpec: test.BaseSpec{
				Name:       "Early upgrade",
				ForkHeight: 2,
			},
			ForkchoiceUpdatedCustomizer: &helper.UpgradeForkchoiceUpdatedVersion{
				ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
					ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
				},
			},
		},
		/*
			TODO: This test is failing because the upgraded version of the ForkchoiceUpdated does not contain the
			      expected fields of the following version.
			ForkchoiceUpdatedOnHeadBlockUpdateTest{
				BaseSpec: test.BaseSpec{
					Name:       "Early upgrade",
					ForkHeight: 2,
				},
				ForkchoiceUpdatedCustomizer: &helper.UpgradeForkchoiceUpdatedVersion{
					ForkchoiceUpdatedCustomizer: &helper.BaseForkchoiceUpdatedCustomizer{
						ExpectedError: globals.UNSUPPORTED_FORK_ERROR,
					},
				},
			},
		*/
	)

	// Payload Execution Tests
	Tests = append(Tests,
		ReExecutePayloadTest{},
		InOrderPayloadExecutionTest{},
		MultiplePayloadsExtendingCanonicalChainTest{
			SetHeadToFirstPayloadReceived: true,
		},
		MultiplePayloadsExtendingCanonicalChainTest{
			SetHeadToFirstPayloadReceived: false,
		},
		NewPayloadOnSyncingClientTest{},
		NewPayloadWithMissingFcUTest{},
	)

	// Invalid Payload Tests
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidParentHash,
		helper.InvalidStateRoot,
		helper.InvalidReceiptsRoot,
		helper.InvalidNumber,
		helper.InvalidGasLimit,
		helper.InvalidGasUsed,
		helper.InvalidTimestamp,
		helper.InvalidPrevRandao,
		helper.RemoveTransaction,
	} {
		for _, syncing := range []bool{false, true} {
			if invalidField == helper.InvalidStateRoot {
				Tests = append(Tests, InvalidPayloadTestCase{
					InvalidField:      invalidField,
					Syncing:           syncing,
					EmptyTransactions: true,
				})
			}
			Tests = append(Tests, InvalidPayloadTestCase{
				InvalidField: invalidField,
				Syncing:      syncing,
			})
		}
	}

	Tests = append(Tests, PayloadBuildAfterInvalidPayloadTest{
		InvalidField: helper.InvalidStateRoot,
	})

	// Invalid Transaction Payload Tests
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidTransactionSignature,
		helper.InvalidTransactionNonce,
		helper.InvalidTransactionGasPrice,
		helper.InvalidTransactionGasTipPrice,
		helper.InvalidTransactionGas,
		helper.InvalidTransactionValue,
		helper.InvalidTransactionChainID,
	} {
		for _, syncing := range []bool{false, true} {
			if invalidField != helper.InvalidTransactionGasTipPrice {
				for _, testTxType := range []helper.TestTransactionType{helper.LegacyTxOnly, helper.DynamicFeeTxOnly} {
					Tests = append(Tests, InvalidPayloadTestCase{
						BaseSpec: test.BaseSpec{
							TestTransactionType: testTxType,
						},
						InvalidField: invalidField,
						Syncing:      syncing,
					})
				}
			} else {
				Tests = append(Tests, InvalidPayloadTestCase{
					BaseSpec: test.BaseSpec{
						TestTransactionType: helper.DynamicFeeTxOnly,
					},
					InvalidField: invalidField,
					Syncing:      syncing,
				})
			}
		}

	}

	// Invalid Transaction ChainID Tests
	Tests = append(Tests,
		InvalidTxChainIDTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.LegacyTxOnly,
			},
		},
		InvalidTxChainIDTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.DynamicFeeTxOnly,
			},
		},
	)

	// Invalid Ancestor Re-Org Tests (Reveal Via NewPayload)
	for _, invalidIndex := range []int{1, 9, 10} {
		for _, emptyTxs := range []bool{false, true} {
			Tests = append(Tests, InvalidMissingAncestorReOrgTest{
				SidechainLength:   10,
				InvalidIndex:      invalidIndex,
				InvalidField:      helper.InvalidStateRoot,
				EmptyTransactions: emptyTxs,
			})
		}
	}

	// Invalid Ancestor Re-Org Tests (Reveal Via Sync)
	spec := test.BaseSpec{
		TimeoutSeconds:   60,
		SlotsToFinalized: big.NewInt(20),
	}
	for _, invalidField := range []helper.InvalidPayloadBlockField{
		helper.InvalidStateRoot,
		helper.InvalidReceiptsRoot,
		// TODO: helper.InvalidNumber, Test is causing a panic on the secondary node, disabling for now.
		helper.InvalidGasLimit,
		helper.InvalidGasUsed,
		helper.InvalidTimestamp,
		// TODO: helper.InvalidPrevRandao, Test consistently fails with Failed to set invalid block: missing trie node.
		helper.RemoveTransaction,
		helper.InvalidTransactionSignature,
		helper.InvalidTransactionNonce,
		helper.InvalidTransactionGas,
		helper.InvalidTransactionGasPrice,
		helper.InvalidTransactionValue,
		// helper.InvalidOmmers, Unsupported now
	} {
		for _, reOrgFromCanonical := range []bool{false, true} {
			invalidIndex := 9
			if invalidField == helper.InvalidReceiptsRoot ||
				invalidField == helper.InvalidGasLimit ||
				invalidField == helper.InvalidGasUsed ||
				invalidField == helper.InvalidTimestamp ||
				invalidField == helper.InvalidPrevRandao {
				invalidIndex = 8
			}
			if invalidField == helper.InvalidStateRoot {
				Tests = append(Tests, InvalidMissingAncestorReOrgSyncTest{
					BaseSpec:           spec,
					InvalidField:       invalidField,
					ReOrgFromCanonical: reOrgFromCanonical,
					EmptyTransactions:  true,
					InvalidIndex:       invalidIndex,
				})
			}
			Tests = append(Tests, InvalidMissingAncestorReOrgSyncTest{
				BaseSpec:           spec,
				InvalidField:       invalidField,
				ReOrgFromCanonical: reOrgFromCanonical,
				InvalidIndex:       invalidIndex,
			})
		}
	}

	// Re-org using the Engine API tests

	// Re-org a transaction out of a block, or into a new block
	Tests = append(Tests,
		TransactionReOrgTest{
			ReorgOut: true,
		},
		TransactionReOrgTest{
			ReorgDifferentBlock: true,
		},
		TransactionReOrgTest{
			ReorgDifferentBlock: true,
			NewPayloadOnRevert:  true,
		},
	)

	// Re-Org back into the canonical chain tests
	Tests = append(Tests,
		ReOrgBackToCanonicalTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(10),
				SlotsToFinalized: big.NewInt(20),
				TimeoutSeconds:   60,
			},
			TransactionPerPayload: 1,
			ReOrgDepth:            5,
		},
		ReOrgBackToCanonicalTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   120,
			},
			TransactionPerPayload:     50,
			ReOrgDepth:                10,
			ExecuteSidePayloadOnReOrg: true,
		},
	)

	// Suggested Fee Recipient Tests
	Tests = append(Tests,
		SuggestedFeeRecipientTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.LegacyTxOnly,
			},
			TransactionCount: 20,
		},
		SuggestedFeeRecipientTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.DynamicFeeTxOnly,
			},
			TransactionCount: 20,
		},
	)

	// PrevRandao opcode tests
	Tests = append(Tests,
		PrevRandaoTransactionTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.LegacyTxOnly,
			},
		},
		PrevRandaoTransactionTest{
			BaseSpec: test.BaseSpec{
				TestTransactionType: helper.DynamicFeeTxOnly,
			},
		},
	)

	// Fork ID Tests
	Tests = append(Tests,
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 0",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         0,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 0, Fork at 1, Current Block 1",
				GenesisTimestamp: pUint64(0),
				MainFork:         config.Paris,
				ForkTime:         1,
			},
			ProduceBlocksBeforePeering: 1,
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 1, Previous Fork at 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         1,
				PreviousForkTime: 1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 2, Previous Fork at 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         2,
				PreviousForkTime: 1,
			},
		},
		ForkIDSpec{
			BaseSpec: test.BaseSpec{
				Name:             "Fork ID: Genesis at 1, Fork at 2, Previous Fork at 1, Current Block 1",
				GenesisTimestamp: pUint64(1),
				MainFork:         config.Paris,
				ForkTime:         2,
				PreviousForkTime: 1,
			},
			ProduceBlocksBeforePeering: 1,
		},
	)
}
