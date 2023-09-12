package suite_engine

import (
	"math/big"

	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

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

var Tests = make([]test.Spec, 0)

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

	// Sidechain re-org tests
	Tests = append(Tests,
		SidechainReOrgTest{},
		ReOrgBackFromSyncingTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
		},
		ReOrgPrevValidatedPayloadOnSideChainTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
		},
		SafeReOrgToSideChainTest{
			BaseSpec: test.BaseSpec{
				SlotsToSafe:      big.NewInt(1),
				SlotsToFinalized: big.NewInt(2),
			},
		},
	)

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
	for genesisTimestamp := uint64(0); genesisTimestamp <= 1; genesisTimestamp++ {
		for forkTime := uint64(0); forkTime <= 2; forkTime++ {
			for prevForkTime := uint64(0); prevForkTime <= forkTime; prevForkTime++ {
				for currentBlock := 0; currentBlock <= 1; currentBlock++ {
					Tests = append(Tests,
						ForkIDSpec{
							BaseSpec: test.BaseSpec{
								MainFork:         config.Paris,
								GenesisTimestamp: pUint64(genesisTimestamp),
								ForkTime:         forkTime,
								PreviousForkTime: prevForkTime,
							},
							ProduceBlocksBeforePeering: currentBlock,
						},
					)
				}
			}
		}
	}
}
