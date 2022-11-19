package suite_withdrawals

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	big0      = new(big.Int)
	big1      = big.NewInt(1)
	Head      *big.Int // Nil
	Pending   = big.NewInt(-2)
	Finalized = big.NewInt(-3)
	Safe      = big.NewInt(-4)
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

var Tests = []test.Spec{
	{
		Name:              "Shanghai Genesis",
		Run:               sanityWithdrawalTestGen(globals.GenesisTimestamp, 1),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp), // Genesis reaches shanghai
	},
	{
		Name:              "Shanghai on Block 1",
		Run:               sanityWithdrawalTestGen(globals.GenesisTimestamp+1, 1),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + 1),
	},
	{
		Name:              "Shanghai on Block 2",
		Run:               sanityWithdrawalTestGen(globals.GenesisTimestamp+2, 1),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + 2),
	},

	/*
		{
			Name:              "Send Withdrawal Pre-Shanghai",
			Run:               sanityWithdrawalTest,
			TTD:               0,
			ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + 5),
		},
	*/
}

// Wait for Shanghai and perform a simple withdrawal.
func sanityWithdrawalTestGen(shanghaiTimestamp int64, postShanghaiBlocks int) func(t *test.Env) {
	return func(t *test.Env) {
		t.CLMock.WaitForTTD()

		// Check if we have pre-Shanghai blocks
		if shanghaiTimestamp > globals.GenesisTimestamp {
			// Check `latest` during all pre-shanghai blocks, none should
			// contain `withdrawalsRoot`, including genesis.

			// Genesis should not contain `withdrawalsRoot` either
			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectWithdrawalsRoot(nil)

			preShanghaiBlocks := shanghaiTimestamp - globals.GenesisTimestamp - 1
			if preShanghaiBlocks > 0 {
				t.CLMock.ProduceBlocks(int(preShanghaiBlocks), clmock.BlockProcessCallbacks{
					OnPayloadProducerSelected: func() {
						// Try to send a ForkchoiceUpdatedV2 before Shanghai
						r := t.TestEngine.TestEngineForkchoiceUpdatedV2(
							&beacon.ForkchoiceStateV1{
								HeadBlockHash: t.CLMock.LatestHeader.Hash(),
							},
							&beacon.PayloadAttributes{
								Timestamp:             t.CLMock.LatestHeader.Time + 1,
								Random:                common.Hash{},
								SuggestedFeeRecipient: common.Address{},
								Withdrawals:           make(types.Withdrawals, 0),
							},
						)
						r.ExpectError()
					},
					OnGetPayload: func() {
						// Send produced payload but try to include non-nil
						// `withdrawals`, it should fail.
						emptyWithdrawalsList := make(types.Withdrawals, 0)
						payloadPlusWithdrawals, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
							Withdrawals: emptyWithdrawalsList,
						})
						if err != nil {
							t.Fatalf("Unable to append withdrawals: %v", err)
						}
						r := t.TestEngine.TestEngine.TestEngineNewPayloadV2(payloadPlusWithdrawals)
						r.ExpectStatus(test.Invalid)
					},
				})
			}

		} else {
			// Genesis is post shanghai, it should contain EmptyWithdrawalsRoot
			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectWithdrawalsRoot(&types.EmptyRootHash)
		}

		/*
			// Verify `latest` block does not have withdrawalsRoot
			p := t.TestEngine.TestBlockByNumber(nil)
			if p.Block.Header().WithdrawalsHash != nil {
				t.Fatalf("FAIL (%s): Pre-Shanghai Block contains withdrawalsRoot: %v",
					t.TestName,
					p.Block.Header().WithdrawalsHash,
				)
			}

			// Perform a withdrawal on the first Shanghai block
			withdrawalAddress := common.HexToAddress("0xAA")
			withdrawalAmount := big.NewInt(100)
			withdrawals := make(types.Withdrawals, 0)
			withdrawals = append(withdrawals, &types.Withdrawal{
				Index:     0,
				Validator: 0,
				Address:   withdrawalAddress,
				Amount:    withdrawalAmount,
			})

			t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
				OnPayloadProducerSelected: func() {
					t.CLMock.SetNextWithdrawals(withdrawals)
				},
			})

			// Check new balance of the withdrawal account
			r := t.TestEngine.TestBalanceAt(withdrawalAddress, nil)
			r.ExpectBalanceEqual(withdrawalAmount)

			// Check withdrawalsRoot of `eth_getBlockByNumber`
			p = t.TestEngine.TestBlockByNumber(nil)
			expectedWithdrawalsRoot := types.DeriveSha(types.Withdrawals(withdrawals), trie.NewStackTrie(nil))
			gotWithdrawalsRoot := p.Block.Header().WithdrawalsHash
			if gotWithdrawalsRoot == nil || *gotWithdrawalsRoot != expectedWithdrawalsRoot {
				t.Fatalf("FAIL (%s): Incorrect withdrawalsRoot: %v!=%v",
					t.TestName,
					gotWithdrawalsRoot,
					expectedWithdrawalsRoot,
				)
			}
		*/
	}
}
