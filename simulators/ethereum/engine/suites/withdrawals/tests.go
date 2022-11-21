// # Test suite for withdrawals tests
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

// List of all withdrawals tests
var Tests = []test.SpecInterface{
	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdrawals Fork On Genesis",
			About: `
			Tests the withdrawals fork happening since genesis (e.g. on a
			testnet).
			`,
		},
		PreWithdrawalsBlocks:  0,
		WithdrawalsBlockCount: 2, // Genesis is a withdrawals block
		WithdrawalsPerBlock:   16,
	},

	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdrawals Fork on Block 1",
			About: `
			Tests the withdrawals fork happening directly after genesis.
			`,
		},
		PreWithdrawalsBlocks:  1, // Only Genesis is Pre-Withdrawals
		WithdrawalsBlockCount: 1,
		WithdrawalsPerBlock:   16,
	},

	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdrawals Fork on Block 2",
			About: `
			Tests the transition to the withdrawals fork after a single block
			has happened.
			`,
		},
		PreWithdrawalsBlocks:  2, // Genesis and Block 1 are Pre-Withdrawals
		WithdrawalsBlockCount: 1,
		WithdrawalsPerBlock:   16,
	},
	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdraw to a single account",
			About: `
			Make multiple withdrawals to a single account.
			`,
		},
		PreWithdrawalsBlocks:     1,
		WithdrawalsBlockCount:    1,
		WithdrawalsPerBlock:      64,
		WithdrawableAccountCount: 1,
	},
	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdraw to two accounts",
			About: `
			Make multiple withdrawals to two different accounts, repeated in
			round-robin.
			Reasoning: There might be a difference in implementation when an
			account appears multiple times in the withdrawals list but the list
			is not in ordered sequence.
			`,
		},
		PreWithdrawalsBlocks:     1,
		WithdrawalsBlockCount:    1,
		WithdrawalsPerBlock:      64,
		WithdrawableAccountCount: 2,
	},
	WithdrawalsBasicSpec{
		Spec: test.Spec{
			Name: "Withdraw to many accounts",
			About: `
			Make multiple withdrawals to 1024 different accounts.
			Execute many blocks this way.
			`,
		},
		PreWithdrawalsBlocks:     1,
		WithdrawalsBlockCount:    16,
		WithdrawalsPerBlock:      1024,
		WithdrawableAccountCount: 1024,
	},
}

// Helper structure used to keep history of the amounts withdrawn to each test account.
type WithdrawalsHistory map[uint64]types.Withdrawals

// Gets an account expected value for a given block, taking into account all
// withdrawals that credited the account.
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

// Get a list of all addresses that were credited by withdrawals on a given block.
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

// Get the withdrawals list for a given block.
func (wh WithdrawalsHistory) GetWithdrawals(block uint64) types.Withdrawals {
	if w, ok := wh[block]; ok {
		return w
	}
	return make(types.Withdrawals, 0)
}

// Withdrawals basic spec:
// Specifies a simple withdrawals test where the withdrawals fork can
type WithdrawalsBasicSpec struct {
	test.Spec
	PreWithdrawalsBlocks     int64 // Number of blocks before withdrawals fork activation
	WithdrawalsBlockCount    int   // Number of blocks on and after withdrawals fork activation
	WithdrawalsPerBlock      int   // Number of withdrawals per block
	WithdrawableAccountCount int64 // Number of accounts to withdraw to (round-robin)
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand (CLMock produces 1 block each second).
func (ws WithdrawalsBasicSpec) GetForkConfig() test.ForkConfig {
	return test.ForkConfig{
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + ws.PreWithdrawalsBlocks),
	}
}

// Wait for Shanghai and perform some withdrawals.
func (ws WithdrawalsBasicSpec) Execute(t *test.Env) {
	t.CLMock.WaitForTTD()

	shanghaiTimestamp := globals.GenesisTimestamp + ws.PreWithdrawalsBlocks

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
		differentAccounts    = ws.WithdrawableAccountCount
		firstWithdrawalBlock = uint64(shanghaiTimestamp - globals.GenesisTimestamp)
	)

	if differentAccounts == 0 {
		// We can't have 0 different accounts to withdraw to
		differentAccounts = int64(16)
	}

	t.CLMock.ProduceBlocks(ws.WithdrawalsBlockCount, clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// Send some withdrawals
			nextWithdrawals := make(types.Withdrawals, 0)
			for i := 0; i < ws.WithdrawalsPerBlock; i++ {
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
						t.CLMock.LatestExecutedPayload.Number-1),
				)
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
						t.CLMock.LatestExecutedPayload.Number),
				)
			}
			// Check the correct withdrawal root on `latest` block
			r := t.TestEngine.TestBlockByNumber(nil)
			expectedWithdrawalsRoot := types.DeriveSha(
				withdrawalsHistory.GetWithdrawals(t.CLMock.LatestExecutedPayload.Number),
				trie.NewStackTrie(nil),
			)
			r.ExpectWithdrawalsRoot(&expectedWithdrawalsRoot)
		},
	})
	// Iterate over balance history of withdrawn accounts using RPC and
	// check that the balances match expected values.
	// Also check one block before the withdrawal took place, verify that
	// withdrawal has not been updated.
	for block := uint64(0); block <= firstWithdrawalBlock+uint64(ws.WithdrawalsBlockCount)-1; block++ {
		for withdrawnAcc := range withdrawnAccounts {
			r := t.TestEngine.TestBalanceAt(withdrawnAcc, big.NewInt(int64(block)))
			r.ExpectBalanceEqual(
				withdrawalsHistory.GetExpectedAccountBalance(
					withdrawnAcc,
					block,
				),
			)
		}
		// Check the correct withdrawal root on past blocks
		r := t.TestEngine.TestBlockByNumber(big.NewInt(int64(block)))
		var expectedWithdrawalsRoot *common.Hash = nil
		t.Logf("INFO (%s): firstWithdrawalBlock=%d, block=%d", t.TestName, firstWithdrawalBlock, block)
		if block >= firstWithdrawalBlock {
			calcWithdrawalsRoot := types.DeriveSha(
				withdrawalsHistory.GetWithdrawals(block),
				trie.NewStackTrie(nil),
			)
			expectedWithdrawalsRoot = &calcWithdrawalsRoot
		}
		r.ExpectWithdrawalsRoot(expectedWithdrawalsRoot)

	}
}
