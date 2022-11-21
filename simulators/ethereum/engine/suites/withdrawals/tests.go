package suite_withdrawals

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
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
		Run:               basicWithdrawalsTestGen(globals.GenesisTimestamp, 1, 16),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp), // Genesis reaches shanghai
	},
	{
		Name:              "Shanghai on Block 1",
		Run:               basicWithdrawalsTestGen(globals.GenesisTimestamp+1, 1, 16),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + 1),
	},
	{
		Name:              "Shanghai on Block 2",
		Run:               basicWithdrawalsTestGen(globals.GenesisTimestamp+2, 1, 16),
		TTD:               0,
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + 2),
	},
}

// Test types
type WithdrawalsHistory map[uint64]types.Withdrawals

func (wh WithdrawalsHistory) GetExpectedAccountBalance(account common.Address, block uint64) *big.Int {
	balance := big.NewInt(0)
	for b := uint64(0); b <= block; b++ {
		if withdrawals, ok := wh[b]; ok {
			for _, withdrawal := range withdrawals {
				if withdrawal.Address == account {
					balance.Add(balance, withdrawal.Amount)
				}
			}
		}
	}
	return balance
}

func (wh WithdrawalsHistory) GetAddressesWithdrawnOnBlock(block uint64) []common.Address {
	addressMap := make(map[common.Address]bool)
	if withdrawals, ok := wh[block]; ok {
		for _, withdrawal := range withdrawals {
			addressMap[withdrawal.Address] = true
		}
	}
	addressList := make([]common.Address, 0)
	for addr := range addressMap {
		addressList = append(addressList, addr)
	}
	return addressList
}

type WithdrawalsTestSpec struct {
	ShanghaiTimestamp     int64
	WithdrawalsBlockCount int
}

// Wait for Shanghai and perform a simple withdrawal.
func basicWithdrawalsTestGen(shanghaiTimestamp int64, postShanghaiBlocks int, withdrawalsPerBlock int) func(t *test.Env) {
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
						// Try to send a ForkchoiceUpdatedV2 with non-null
						// withdrawals before Shanghai
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
					OnNewPayloadBroadcast: func() {
						// We sent a pre-shanghai FCU.
						// Keep expecting `nil` until Shanghai.
						r := t.TestEngine.TestBlockByNumber(nil)
						r.ExpectWithdrawalsRoot(nil)
					},
				})
			}

		} else {
			// Genesis is post shanghai, it should contain EmptyWithdrawalsRoot
			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectWithdrawalsRoot(&types.EmptyRootHash)
		}

		// Produce requested post-shanghai blocks
		// (At least 1 block will be produced after this procedure ends).
		var (
			withdrawnAccounts    = make(map[common.Address]bool)
			withdrawalsHistory   = make(WithdrawalsHistory)
			startAccount         = int64(0x100)
			nextAccount          = startAccount
			nextIndex            = uint64(0)
			differentAccounts    = int64(16)
			firstWithdrawalBlock = t.CLMock.LatestExecutedPayload.Number + 1
			lastWithdrawals      types.Withdrawals
		)

		t.CLMock.ProduceBlocks(postShanghaiBlocks, clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// Send some withdrawals
				nextWithdrawals := make(types.Withdrawals, 0)
				for i := 0; i < withdrawalsPerBlock; i++ {
					nextWithdrawal := &types.Withdrawal{
						Index:     nextIndex,
						Validator: uint64(nextIndex),
						Address:   common.BigToAddress(big.NewInt(nextAccount)),
						Amount:    big.NewInt(1),
					}
					nextWithdrawals = append(nextWithdrawals, nextWithdrawal)
					withdrawnAccounts[common.BigToAddress(big.NewInt(nextAccount))] = true
					nextIndex++
					nextAccount = (((nextAccount - startAccount) + 1) % differentAccounts) + startAccount
				}
				t.CLMock.NextWithdrawals = nextWithdrawals
				withdrawalsHistory[t.CLMock.LatestHeader.Number.Uint64()+1] = nextWithdrawals
				lastWithdrawals = nextWithdrawals
			},
			OnGetPayload: func() {
				// Send new payload with `withdrawals=null` and expect error
			},
			OnNewPayloadBroadcast: func() {
				// Check withdrawal addresses and verify withdrawal balances
				// have not yet been applied
				for _, addr := range withdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
					// Test balance at `latest`, which should not yet have the
					// withdrawal applied.
					r := t.TestEngine.TestBalanceAt(addr, nil)
					r.ExpectBalanceEqual(
						withdrawalsHistory.GetExpectedAccountBalance(
							addr,
							t.CLMock.LatestExecutedPayload.Number-1))
				}
			},
			OnForkchoiceBroadcast: func() {
				// Check withdrawal addresses and verify withdrawal balances
				// have been applied
				for _, addr := range withdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
					// Test balance at `latest`, which should not yet have the
					// withdrawal applied.
					r := t.TestEngine.TestBalanceAt(addr, nil)
					r.ExpectBalanceEqual(
						withdrawalsHistory.GetExpectedAccountBalance(
							addr,
							t.CLMock.LatestExecutedPayload.Number))
				}
				// Check the correct withdrawal root on `latest` block
				r := t.TestEngine.TestBlockByNumber(nil)
				expectedWithdrawalsRoot := types.DeriveSha(lastWithdrawals, trie.NewStackTrie(nil))
				r.ExpectWithdrawalsRoot(&expectedWithdrawalsRoot)
			},
		})
		// Iterate over balance history of withdrawn accounts using RPC and
		// check that the balances match expected values.
		// Also check one block before the withdrawal took place, verify that
		// withdrawal has not been updated.
		for block := firstWithdrawalBlock - 1; block <= firstWithdrawalBlock+uint64(postShanghaiBlocks)-1; block++ {
			for withdrawnAcc := range withdrawnAccounts {
				r := t.TestEngine.TestBalanceAt(withdrawnAcc, big.NewInt(int64(block)))
				r.ExpectBalanceEqual(
					withdrawalsHistory.GetExpectedAccountBalance(
						withdrawnAcc,
						block))
			}
			// Check the correct withdrawal root on past blocks
			r := t.TestEngine.TestBlockByNumber(big.NewInt(int64(block)))
			var expectedWithdrawalsRoot *common.Hash = nil
			if block >= firstWithdrawalBlock {
				calcWithdrawalsRoot := types.DeriveSha(lastWithdrawals, trie.NewStackTrie(nil))
				expectedWithdrawalsRoot = &calcWithdrawalsRoot
			}
			r.ExpectWithdrawalsRoot(expectedWithdrawalsRoot)

		}
	}
}
