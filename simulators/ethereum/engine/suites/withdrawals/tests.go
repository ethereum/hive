// # Test suite for withdrawals tests
package suite_withdrawals

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
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
	WithdrawalsBaseSpec{
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

	WithdrawalsBaseSpec{
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

	WithdrawalsBaseSpec{
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
	WithdrawalsBaseSpec{
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
	WithdrawalsBaseSpec{
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
	WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdraw many accounts",
			About: `
			Make multiple withdrawals to 1024 different accounts.
			Execute many blocks this way.
			`,
			TimeoutSeconds: 120,
		},
		PreWithdrawalsBlocks:     1,
		WithdrawalsBlockCount:    16,
		WithdrawalsPerBlock:      1024,
		WithdrawableAccountCount: 1024,
	},
	// Sync Tests
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 1
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync and verify withdrawn account's balance
			`,
			},
			PreWithdrawalsBlocks:     1,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
		},
		SyncSteps: 1,
	},
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Genesis - Single Withdrawal Account",
				About: `
			- Spawn a first client, with Withdrawals since genesis
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync and verify withdrawn account's balance
			`,
			},
			PreWithdrawalsBlocks:     0,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
		},
		SyncSteps: 1,
	},
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 2 - Single Withdrawal Account",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
			`,
			},
			PreWithdrawalsBlocks:     2,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
		},
		SyncSteps: 1,
	},
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
			`,
				TimeoutSeconds: 300,
			},
			PreWithdrawalsBlocks:     2,
			WithdrawalsBlockCount:    128,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		SyncSteps: 1,
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

// Get the withdrawn accounts list until a given block height.
func (wh WithdrawalsHistory) GetWithdrawnAccounts(blockHeight uint64) map[common.Address]*big.Int {
	accounts := make(map[common.Address]*big.Int)
	for block := uint64(0); block <= blockHeight; block++ {
		if withdrawals, ok := wh[block]; ok {
			for _, withdrawal := range withdrawals {
				if currentBalance, ok2 := accounts[withdrawal.Address]; ok2 {
					currentBalance.Add(currentBalance, withdrawal.Amount)
				} else {
					accounts[withdrawal.Address] = new(big.Int).Set(withdrawal.Amount)
				}
			}
		}
	}
	return accounts
}

// Verify all withdrawals on a client at a given height
func (wh WithdrawalsHistory) VerifyWithdrawals(block uint64, rpcBlock *big.Int, testEngine *test.TestEngineClient) {
	accounts := wh.GetWithdrawnAccounts(block)
	for account, expectedBalance := range accounts {
		r := testEngine.TestBalanceAt(account, rpcBlock)
		r.ExpectBalanceEqual(expectedBalance)
	}
}

// Withdrawals base spec:
// Specifies a simple withdrawals test where the withdrawals fork can happen
// on genesis or afterwards.
type WithdrawalsBaseSpec struct {
	test.Spec
	PreWithdrawalsBlocks     int64              // Number of blocks before withdrawals fork activation
	WithdrawalsBlockCount    int                // Number of blocks on and after withdrawals fork activation
	WithdrawalsPerBlock      int                // Number of withdrawals per block
	WithdrawableAccountCount int64              // Number of accounts to withdraw to (round-robin)
	WithdrawalsHistory       WithdrawalsHistory // Internal withdrawals history that keeps track of all withdrawals
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand (CLMock produces 1 block each second).
func (ws WithdrawalsBaseSpec) GetForkConfig() test.ForkConfig {
	return test.ForkConfig{
		ShanghaiTimestamp: big.NewInt(globals.GenesisTimestamp + ws.PreWithdrawalsBlocks),
	}
}

func (ws WithdrawalsBaseSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)

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
		startAccount         = int64(0x1000)
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
			ws.WithdrawalsHistory[t.CLMock.LatestHeader.Number.Uint64()+1] = nextWithdrawals
		},
		OnGetPayload: func() {
			// Send new payload with `withdrawals=null` and expect error
		},
		OnNewPayloadBroadcast: func() {
			// Check withdrawal addresses and verify withdrawal balances
			// have not yet been applied
			for _, addr := range ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
				// Test balance at `latest`, which should not yet have the
				// withdrawal applied.
				r := t.TestEngine.TestBalanceAt(addr, nil)
				r.ExpectBalanceEqual(
					ws.WithdrawalsHistory.GetExpectedAccountBalance(
						addr,
						t.CLMock.LatestExecutedPayload.Number-1),
				)
			}
		},
		OnForkchoiceBroadcast: func() {
			// Check withdrawal addresses and verify withdrawal balances
			// have been applied
			for _, addr := range ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
				// Test balance at `latest`, which should not yet have the
				// withdrawal applied.
				r := t.TestEngine.TestBalanceAt(addr, nil)
				r.ExpectBalanceEqual(
					ws.WithdrawalsHistory.GetExpectedAccountBalance(
						addr,
						t.CLMock.LatestExecutedPayload.Number),
				)
			}
			// Check the correct withdrawal root on `latest` block
			r := t.TestEngine.TestBlockByNumber(nil)
			expectedWithdrawalsRoot := types.DeriveSha(
				ws.WithdrawalsHistory.GetWithdrawals(t.CLMock.LatestExecutedPayload.Number),
				trie.NewStackTrie(nil),
			)
			r.ExpectWithdrawalsRoot(&expectedWithdrawalsRoot)
		},
	})
	// Iterate over balance history of withdrawn accounts using RPC and
	// check that the balances match expected values.
	// Also check one block before the withdrawal took place, verify that
	// withdrawal has not been updated.
	for block := uint64(0); block <= t.CLMock.LatestExecutedPayload.Number; block++ {
		ws.WithdrawalsHistory.VerifyWithdrawals(block, big.NewInt(int64(block)), t.TestEngine)

		// Check the correct withdrawal root on past blocks
		r := t.TestEngine.TestBlockByNumber(big.NewInt(int64(block)))
		var expectedWithdrawalsRoot *common.Hash = nil
		t.Logf("INFO (%s): firstWithdrawalBlock=%d, block=%d", t.TestName, firstWithdrawalBlock, block)
		if block >= firstWithdrawalBlock {
			calcWithdrawalsRoot := types.DeriveSha(
				ws.WithdrawalsHistory.GetWithdrawals(block),
				trie.NewStackTrie(nil),
			)
			expectedWithdrawalsRoot = &calcWithdrawalsRoot
		}
		r.ExpectWithdrawalsRoot(expectedWithdrawalsRoot)

	}

	// Verify on `latest`
	ws.WithdrawalsHistory.VerifyWithdrawals(t.CLMock.LatestExecutedPayload.Number, nil, t.TestEngine)
}

// Withdrawals sync spec:
// Specifies a withdrawals test where the withdrawals happen and then a
// client needs to sync and apply the withdrawals.
type WithdrawalsSyncSpec struct {
	WithdrawalsBaseSpec
	SyncSteps      int  // Sync block chunks that will be passed as head through FCUs to the syncing client
	SyncShouldFail bool //
}

func (ws WithdrawalsSyncSpec) Execute(t *test.Env) {
	// Do the base withdrawal test first
	ws.WithdrawalsBaseSpec.Execute(t)

	// Spawn a secondary client which will need to sync to the primary client
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.ClientParams, t.ClientFiles, t.Engine)

	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(secondaryEngine)

	if ws.SyncSteps > 1 {
		// TODO
	} else {
		// Send the FCU to trigger sync on the secondary client
	loop:
		for {
			select {
			case <-t.TimeoutContext.Done():
				t.Fatalf("FAIL (%s): Timeout while waiting for secondary client to sync", t.TestName)
			case <-time.After(time.Second):
				secondaryEngineTest.TestEngineNewPayloadV2(
					&t.CLMock.LatestExecutedPayload,
				)
				r := secondaryEngineTest.TestEngineForkchoiceUpdatedV2(
					&t.CLMock.LatestForkchoice,
					&beacon.PayloadAttributes{},
				)
				if r.Response.PayloadStatus.Status == test.Valid {
					break loop
				}
			}
		}

		ws.WithdrawalsHistory.VerifyWithdrawals(t.CLMock.LatestHeader.Nonce.Uint64(), nil, secondaryEngineTest)

	}
}

// Withdrawals re-org spec:
// Specifies a withdrawals test where the withdrawals re-org can happen
// even to a point before withdrawals were enabled, or simply to a previous
// withdrawals block.
type WithdrawalsReorgSpec struct {
	WithdrawalsBaseSpec
	ReOrgOriginBlockHeight uint64 // Height of the block where the re-org will happen
	ReOrgDestBlockHeight   uint64 // Height of the block to which the re-org will rollback
}
