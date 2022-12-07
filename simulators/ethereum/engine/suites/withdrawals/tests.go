// # Test suite for withdrawals tests
package suite_withdrawals

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
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
		WithdrawalsForkHeight: 0,
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
		WithdrawalsForkHeight: 1, // Only Genesis is Pre-Withdrawals
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
		WithdrawalsForkHeight: 2, // Genesis and Block 1 are Pre-Withdrawals
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
		WithdrawalsForkHeight:    1,
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
		WithdrawalsForkHeight:    1,
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
		WithdrawalsForkHeight:    1,
		WithdrawalsBlockCount:    4,
		WithdrawalsPerBlock:      1024,
		WithdrawableAccountCount: 1024,
	},

	WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdraw zero amount",
			About: `
			Make multiple withdrawals with amount==0.
			`,
		},
		WithdrawalsForkHeight:    1,
		WithdrawalsBlockCount:    1,
		WithdrawalsPerBlock:      64,
		WithdrawableAccountCount: 2,
		WithdrawAmounts: []*big.Int{
			common.Big0,
			common.Big1,
		},
	},

	WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Empty Withdrawals",
			About: `
			Produce withdrawals block with zero withdrawals.
			`,
		},
		WithdrawalsForkHeight: 1,
		WithdrawalsBlockCount: 1,
		WithdrawalsPerBlock:   0,
	},

	// Sync Tests
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account - No Transactions",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 1
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync and verify withdrawn account's balance
			`,
				TimeoutSeconds: 6000,
			},
			WithdrawalsForkHeight:    1,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
			TransactionsPerBlock:     common.Big0,
		},
		SyncSteps: 1,
	},
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
			WithdrawalsForkHeight:    1,
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
			WithdrawalsForkHeight:    0,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
		},
		SyncSteps: 1,
	},
	WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 2 - Single Withdrawal Account - No Transactions",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to a single account 16 times each block for 8 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
			`,
			},
			WithdrawalsForkHeight:    2,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1,
			TransactionsPerBlock:     common.Big0,
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
			WithdrawalsForkHeight:    2,
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
			WithdrawalsForkHeight:    2,
			WithdrawalsBlockCount:    128,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		SyncSteps: 1,
	},

	//Re-Org tests
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 1 Block Re-Org",
				About: `
				Tests a simple 1 block re-org 
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 1,
		ReOrgViaSync:    false,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload",
				About: `
				Tests a 8 block re-org using NewPayload
				Re-org does not change withdrawals fork height
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 8,
		ReOrgViaSync:    false,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org, Sync",
				About: `
				Tests a 8 block re-org using NewPayload
				Re-org does not change withdrawals fork height
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 8,
		ReOrgViaSync:    true,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org NewPayload",
				About: `
				Tests a 10 block re-org using NewPayload
				Re-org does not change withdrawals fork height, but changes
				the payload at the height of the fork
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount: 10,
		ReOrgViaSync:    false,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Re-org does not change withdrawals fork height, but changes
				the payload at the height of the fork
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount: 10,
		ReOrgViaSync:    true,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org",
				About: `
				Tests a 10 block re-org using NewPayload
				Sidechain reaches withdrawals fork at a lower block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            false,
		SidechainTimeIncrements: 2,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Sidechain reaches withdrawals fork at a lower block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            true,
		SidechainTimeIncrements: 2,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org",
				About: `
				Tests a 10 block re-org using NewPayload
				Sidechain reaches withdrawals fork at a higher block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
			TimeIncrements:        2,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            false,
		SidechainTimeIncrements: 1,
	},
	WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Sidechain reaches withdrawals fork at a higher block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
			TimeIncrements:        2,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            true,
		SidechainTimeIncrements: 1,
	},
	// TODO: REORG SYNC WHERE SYNCED BLOCKS HAVE WITHDRAWALS BEFORE TIME
}

// Helper structure used to keep history of the amounts withdrawn to each test account.
type WithdrawalsHistory map[uint64]types.Withdrawals

// Gets an account expected value for a given block, taking into account all
// withdrawals that credited the account.
func (wh WithdrawalsHistory) GetExpectedAccountBalance(account common.Address, block uint64) *big.Int {
	balance := big.NewInt(0)
	for b := uint64(0); b <= block; b++ {
		if withdrawals, ok := wh[b]; ok && withdrawals != nil {
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
	if withdrawals, ok := wh[block]; ok && withdrawals != nil {
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
	if w, ok := wh[block]; ok && w != nil {
		return w
	}
	return make(types.Withdrawals, 0)
}

// Get the withdrawn accounts list until a given block height.
func (wh WithdrawalsHistory) GetWithdrawnAccounts(blockHeight uint64) map[common.Address]*big.Int {
	accounts := make(map[common.Address]*big.Int)
	for block := uint64(0); block <= blockHeight; block++ {
		if withdrawals, ok := wh[block]; ok && withdrawals != nil {
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
		// All withdrawals account have a bytecode that unconditionally set the
		// zero storage key to one on EVM execution.
		// Withdrawals must not trigger EVM so we expect zero.
		s := testEngine.TestStorageAt(account, common.BigToHash(common.Big0), rpcBlock)
		s.ExpectBigIntStorageEqual(common.Big0)
	}
}

// Create a new copy of the withdrawals history
func (wh WithdrawalsHistory) Copy() WithdrawalsHistory {
	result := make(WithdrawalsHistory)
	for k, v := range wh {
		result[k] = v
	}
	return result
}

// Withdrawals base spec:
// Specifies a simple withdrawals test where the withdrawals fork can happen
// on genesis or afterwards.
type WithdrawalsBaseSpec struct {
	test.Spec
	TimeIncrements           uint64             // Timestamp increments per block throughout the test
	WithdrawalsForkHeight    uint64             // Withdrawals activation fork height
	WithdrawalsBlockCount    uint64             // Number of blocks on and after withdrawals fork activation
	WithdrawalsPerBlock      uint64             // Number of withdrawals per block
	WithdrawableAccountCount uint64             // Number of accounts to withdraw to (round-robin)
	WithdrawalsHistory       WithdrawalsHistory // Internal withdrawals history that keeps track of all withdrawals
	WithdrawAmounts          []*big.Int         // Amounts of withdrawn wei on each withdrawal (round-robin)
	TransactionsPerBlock     *big.Int           // Amount of test transactions to include in withdrawal blocks
	SkipBaseVerifications    bool               // For code reuse of the base spec procedure
}

// Get the per-block timestamp increments configured for this test
func (ws WithdrawalsBaseSpec) GetBlockTimeIncrements() uint64 {
	if ws.TimeIncrements == 0 {
		return 1
	}
	return ws.TimeIncrements
}

// Timestamp delta between genesis and the withdrawals fork
func (ws WithdrawalsBaseSpec) GetWithdrawalsGenesisTimeDelta() uint64 {
	return ws.WithdrawalsForkHeight * ws.GetBlockTimeIncrements()
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (ws WithdrawalsBaseSpec) GetWithdrawalsForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + ws.GetWithdrawalsGenesisTimeDelta()
}

// Generates the fork config, including withdrawals fork timestamp.
func (ws WithdrawalsBaseSpec) GetForkConfig() test.ForkConfig {
	return test.ForkConfig{
		ShanghaiTimestamp: big.NewInt(int64(ws.GetWithdrawalsForkTime())),
	}
}

// Adds bytecode that unconditionally sets an storage key to specified account range
func AddUnconditionalBytecode(g *core.Genesis, start *big.Int, end *big.Int) {
	for ; start.Cmp(end) <= 0; start.Add(start, common.Big1) {
		accountAddress := common.BigToAddress(start)
		// Bytecode to unconditionally set a storage key
		g.Alloc[accountAddress] = core.GenesisAccount{
			Code: []byte{
				0x60,
				0x01,
				0x60,
				0x00,
				0x55,
				0x00,
			}, // sstore(0, 1)
			Nonce:   0,
			Balance: common.Big0,
		}
	}
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (ws WithdrawalsBaseSpec) GetGenesis() *core.Genesis {
	genesis := ws.Spec.GetGenesis()
	startAccount := big.NewInt(0x1000)
	endAccount := big.NewInt(0x1000 + int64(ws.GetWithdrawableAccountCount()) - 1)
	AddUnconditionalBytecode(genesis, startAccount, endAccount)
	return genesis
}

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (ws WithdrawalsBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(ws.GetBlockTimeIncrements()))
}

// Number of blocks to be produced (not counting genesis) before withdrawals
// fork.
func (ws WithdrawalsBaseSpec) GetPreWithdrawalsBlockCount() uint64 {
	if ws.WithdrawalsForkHeight == 0 {
		return 0
	} else {
		return ws.WithdrawalsForkHeight - 1
	}
}

// Number of payloads to be produced (pre and post withdrawals) during the entire test
func (ws WithdrawalsBaseSpec) GetTotalPayloadCount() uint64 {
	return ws.GetPreWithdrawalsBlockCount() + ws.WithdrawalsBlockCount
}

func (ws WithdrawalsBaseSpec) GetWithdrawableAccountCount() uint64 {
	if ws.WithdrawableAccountCount == 0 {
		// Withdraw to 16 accounts by default
		return 16
	}
	return ws.WithdrawableAccountCount
}

// Generates a list of withdrawals based on current configuration
func (ws WithdrawalsBaseSpec) GenerateWithdrawalsForBlock(nextIndex uint64, startAccount *big.Int) (types.Withdrawals, uint64) {
	differentAccounts := ws.GetWithdrawableAccountCount()
	withdrawAmounts := ws.WithdrawAmounts
	if withdrawAmounts == nil {
		withdrawAmounts = []*big.Int{
			big.NewInt(1),
		}
	}

	nextWithdrawals := make(types.Withdrawals, 0)
	for i := uint64(0); i < ws.WithdrawalsPerBlock; i++ {
		nextAccount := new(big.Int).Set(startAccount)
		nextAccount.Add(nextAccount, big.NewInt(int64(nextIndex%differentAccounts)))
		nextWithdrawal := &types.Withdrawal{
			Index:     nextIndex,
			Validator: nextIndex,
			Address:   common.BigToAddress(nextAccount),
			Amount:    withdrawAmounts[int(nextIndex)%len(withdrawAmounts)],
		}
		nextWithdrawals = append(nextWithdrawals, nextWithdrawal)
		nextIndex++
	}
	return nextWithdrawals, nextIndex
}

func (ws WithdrawalsBaseSpec) GetTransactionCountPerPayload() uint64 {
	if ws.TransactionsPerBlock == nil {
		return 16
	}
	return ws.TransactionsPerBlock.Uint64()
}

// Base test case execution procedure for withdrawals
func (ws WithdrawalsBaseSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)

	t.CLMock.WaitForTTD()

	// Check if we have pre-Shanghai blocks
	if ws.GetWithdrawalsForkTime() > uint64(globals.GenesisTimestamp) {
		// Check `latest` during all pre-shanghai blocks, none should
		// contain `withdrawalsRoot`, including genesis.

		// Genesis should not contain `withdrawalsRoot` either
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectWithdrawalsRoot(nil)
	} else {
		// Genesis is post shanghai, it should contain EmptyWithdrawalsRoot
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectWithdrawalsRoot(&types.EmptyRootHash)
	}

	// Produce any blocks necessary to reach withdrawals fork
	t.CLMock.ProduceBlocks(int(ws.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{
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
			r.ExpectLatestValidHash(&t.CLMock.LatestExecutedPayload.BlockHash)
		},
		OnNewPayloadBroadcast: func() {
			// We sent a pre-shanghai FCU.
			// Keep expecting `nil` until Shanghai.
			r := t.TestEngine.TestBlockByNumber(nil)
			r.ExpectWithdrawalsRoot(nil)
		},
	})

	// Produce requested post-shanghai blocks
	// (At least 1 block will be produced after this procedure ends).
	var (
		startAccount = big.NewInt(0x1000)
		nextIndex    = uint64(0)
	)

	t.CLMock.ProduceBlocks(int(ws.WithdrawalsBlockCount), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			// Send some withdrawals
			t.CLMock.NextWithdrawals, nextIndex = ws.GenerateWithdrawalsForBlock(nextIndex, startAccount)
			ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
			// Send some transactions
			for i := uint64(0); i < ws.GetTransactionCountPerPayload(); i++ {
				_, err := helper.SendNextTransaction(t.TestContext, t.CLMock.NextBlockProducer, globals.PrevRandaoContractAddr, common.Big1, nil, t.TestTransactionType)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			}
		},
		OnGetPayload: func() {
			// TODO: Send new payload with `withdrawals=null` and expect error
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
		if block >= ws.WithdrawalsForkHeight {
			calcWithdrawalsRoot := types.DeriveSha(
				ws.WithdrawalsHistory.GetWithdrawals(block),
				trie.NewStackTrie(nil),
			)
			expectedWithdrawalsRoot = &calcWithdrawalsRoot
		}
		t.Logf("INFO (%s): Verifying withdrawals root on block %d (%s) to be %s", t.TestName, block, t.CLMock.ExecutedPayloadHistory[block].BlockHash, expectedWithdrawalsRoot)
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
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
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
					nil,
				)
				if r.Response.PayloadStatus.Status == test.Valid {
					break loop
				}
				if r.Response.PayloadStatus.Status == test.Invalid {
					t.Fatalf("FAIL (%s): Syncing client rejected valid chain: %s", t.TestName, r.Response)
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

	ReOrgBlockCount         uint64 // How many blocks the re-org will replace, including the head
	ReOrgViaSync            bool   // Whether the client should fetch the sidechain by syncing from the secondary client
	SidechainTimeIncrements uint64
}

func (ws WithdrawalsReorgSpec) GetSidechainSplitHeight() uint64 {
	if ws.ReOrgBlockCount > ws.GetTotalPayloadCount() {
		panic("invalid payload/re-org configuration")
	}
	return ws.GetTotalPayloadCount() + 1 - ws.ReOrgBlockCount
}

func (ws WithdrawalsReorgSpec) GetSidechainBlockTimeIncrements() uint64 {
	if ws.SidechainTimeIncrements == 0 {
		return ws.GetBlockTimeIncrements()
	}
	return ws.SidechainTimeIncrements
}

func (ws WithdrawalsReorgSpec) GetSidechainWithdrawalsForkHeight() uint64 {
	if ws.GetSidechainBlockTimeIncrements() != ws.GetBlockTimeIncrements() {
		// Block timestamp increments in both chains are different so need to calculate different heights, only if split happens before fork
		if ws.GetSidechainSplitHeight() == 0 {
			// We cannot split by having two different genesis blocks.
			panic("invalid sidechain split height")
		}
		if ws.GetSidechainSplitHeight() <= ws.WithdrawalsForkHeight {
			// We need to calculate the height of the fork on the sidechain
			sidechainSplitBlockTimestamp := ((ws.GetSidechainSplitHeight() - 1) * ws.GetBlockTimeIncrements())
			remainingTime := (ws.GetWithdrawalsGenesisTimeDelta() - sidechainSplitBlockTimestamp)
			if remainingTime == 0 {
				return ws.GetSidechainSplitHeight()
			}
			return ((remainingTime - 1) / ws.SidechainTimeIncrements) + ws.GetSidechainSplitHeight()

		}
	}
	return ws.WithdrawalsForkHeight
}

func (ws WithdrawalsReorgSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)

	t.CLMock.WaitForTTD()

	// Spawn a secondary client which will produce the sidechain
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	// t.CLMock.AddEngineClient(secondaryEngine)

	var (
		canonicalStartAccount       = big.NewInt(0x1000)
		canonicalNextIndex          = uint64(0)
		sidechainStartAccount       = new(big.Int).SetBit(common.Big0, 160, 1)
		sidechainNextIndex          = uint64(0)
		sidechainWithdrawalsHistory = make(WithdrawalsHistory)
		sidechain                   = make(map[uint64]*beacon.ExecutableData)
		sidechainPayloadId          *beacon.PayloadID
	)

	// Sidechain withdraws on the max account value range 0xffffffffffffffffffffffffffffffffffffffff
	sidechainStartAccount.Sub(sidechainStartAccount, big.NewInt(int64(ws.GetWithdrawableAccountCount())+1))

	t.CLMock.ProduceBlocks(int(ws.GetPreWithdrawalsBlockCount()+ws.WithdrawalsBlockCount), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			t.CLMock.NextWithdrawals = nil

			if t.CLMock.CurrentPayloadNumber >= ws.WithdrawalsForkHeight {
				// Prepare some withdrawals
				t.CLMock.NextWithdrawals, canonicalNextIndex = ws.GenerateWithdrawalsForBlock(canonicalNextIndex, canonicalStartAccount)
				ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
			}

			if t.CLMock.CurrentPayloadNumber >= ws.GetSidechainSplitHeight() {
				// We have split
				if t.CLMock.CurrentPayloadNumber >= ws.GetSidechainWithdrawalsForkHeight() {
					// And we are past the withdrawals fork on the sidechain
					sidechainWithdrawalsHistory[t.CLMock.CurrentPayloadNumber], sidechainNextIndex = ws.GenerateWithdrawalsForBlock(sidechainNextIndex, sidechainStartAccount)
				} // else nothing to do
			} else {
				// We have not split
				sidechainWithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
				sidechainNextIndex = canonicalNextIndex
			}

		},
		OnRequestNextPayload: func() {
			// Send transactions to be included in the payload
			for i := uint64(0); i < ws.GetTransactionCountPerPayload(); i++ {
				tx, err := helper.SendNextTransaction(t.TestContext, t.CLMock.NextBlockProducer, globals.PrevRandaoContractAddr, common.Big1, nil, t.TestTransactionType)
				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
				// Error will be ignored here since the tx could have been already relayed
				secondaryEngine.SendTransaction(t.TestContext, tx)
			}

			if t.CLMock.CurrentPayloadNumber >= ws.GetSidechainSplitHeight() {
				// Also request a payload from the sidechain
				fcU := beacon.ForkchoiceStateV1{
					HeadBlockHash: t.CLMock.LatestForkchoice.HeadBlockHash,
				}

				if t.CLMock.CurrentPayloadNumber > ws.GetSidechainSplitHeight() {
					if lastSidePayload, ok := sidechain[t.CLMock.CurrentPayloadNumber-1]; !ok {
						panic("sidechain payload not found")
					} else {
						fcU.HeadBlockHash = lastSidePayload.BlockHash
					}
				}

				var version int
				pAttributes := beacon.PayloadAttributes{
					Random:                t.CLMock.LatestPayloadAttributes.Random,
					SuggestedFeeRecipient: t.CLMock.LatestPayloadAttributes.SuggestedFeeRecipient,
				}
				if t.CLMock.CurrentPayloadNumber > ws.GetSidechainSplitHeight() {
					pAttributes.Timestamp = sidechain[t.CLMock.CurrentPayloadNumber-1].Timestamp + uint64(ws.GetSidechainBlockTimeIncrements())
				} else if t.CLMock.CurrentPayloadNumber == ws.GetSidechainSplitHeight() {
					pAttributes.Timestamp = t.CLMock.LatestHeader.Time + uint64(ws.GetSidechainBlockTimeIncrements())
				} else {
					pAttributes.Timestamp = t.CLMock.LatestPayloadAttributes.Timestamp
				}
				if t.CLMock.CurrentPayloadNumber >= ws.GetSidechainWithdrawalsForkHeight() {
					// Withdrawals
					version = 2
					pAttributes.Withdrawals = sidechainWithdrawalsHistory[t.CLMock.CurrentPayloadNumber]
				} else {
					// No withdrawals
					version = 1
				}

				t.Logf("INFO (%s): Requesting sidechain payload %d: %v", t.TestName, t.CLMock.CurrentPayloadNumber, pAttributes)

				r := secondaryEngineTest.TestEngineForkchoiceUpdated(&fcU, &pAttributes, version)
				r.ExpectNoError()
				r.ExpectPayloadStatus(test.Valid)
				if r.Response.PayloadID == nil {
					t.Fatalf("FAIL (%s): Unable to get a payload ID on the sidechain", t.TestName)
				}
				sidechainPayloadId = r.Response.PayloadID
			}
		},
		OnGetPayload: func() {
			var (
				version int
				payload *beacon.ExecutableData
			)
			if t.CLMock.CurrentPayloadNumber >= ws.GetSidechainWithdrawalsForkHeight() {
				version = 2
			} else {
				version = 1
			}
			if t.CLMock.LatestPayloadBuilt.Number >= ws.GetSidechainSplitHeight() {
				// This payload is built by the secondary client, hence need to manually fetch it here
				r := secondaryEngineTest.TestEngineGetPayload(sidechainPayloadId, version)
				r.ExpectNoError()
				payload = &r.Payload
				sidechain[payload.Number] = payload
			} else {
				// This block is part of both chains, simply forward it to the secondary client
				payload = &t.CLMock.LatestPayloadBuilt
			}
			r := secondaryEngineTest.TestEngineNewPayload(payload, version)
			r.ExpectStatus(test.Valid)
			p := secondaryEngineTest.TestEngineForkchoiceUpdated(
				&beacon.ForkchoiceStateV1{
					HeadBlockHash: payload.BlockHash,
				},
				nil,
				version,
			)
			p.ExpectPayloadStatus(test.Valid)
		},
	})

	sidechainHeight := t.CLMock.LatestExecutedPayload.Number

	if ws.WithdrawalsForkHeight < ws.GetSidechainWithdrawalsForkHeight() {
		// This means the canonical chain forked before the sidechain.
		// Therefore we need to produce more sidechain payloads to reach
		// at least`ws.WithdrawalsBlockCount` withdrawals payloads produced on
		// the sidechain.
		for i := uint64(0); i < ws.GetSidechainWithdrawalsForkHeight()-ws.WithdrawalsForkHeight; i++ {
			sidechainWithdrawalsHistory[sidechainHeight+1], sidechainNextIndex = ws.GenerateWithdrawalsForBlock(sidechainNextIndex, sidechainStartAccount)
			pAttributes := beacon.PayloadAttributes{
				Timestamp:             sidechain[sidechainHeight].Timestamp + ws.GetSidechainBlockTimeIncrements(),
				Random:                t.CLMock.LatestPayloadAttributes.Random,
				SuggestedFeeRecipient: t.CLMock.LatestPayloadAttributes.SuggestedFeeRecipient,
				Withdrawals:           sidechainWithdrawalsHistory[sidechainHeight+1],
			}
			r := secondaryEngineTest.TestEngineForkchoiceUpdatedV2(&beacon.ForkchoiceStateV1{
				HeadBlockHash: sidechain[sidechainHeight].BlockHash,
			}, &pAttributes)
			r.ExpectPayloadStatus(test.Valid)
			time.Sleep(time.Second)
			p := secondaryEngineTest.TestEngineGetPayloadV2(r.Response.PayloadID)
			p.ExpectNoError()
			s := secondaryEngineTest.TestEngineNewPayloadV2(&p.Payload)
			s.ExpectStatus(test.Valid)
			q := secondaryEngineTest.TestEngineForkchoiceUpdatedV2(
				&beacon.ForkchoiceStateV1{
					HeadBlockHash: p.Payload.BlockHash,
				},
				nil,
			)
			q.ExpectPayloadStatus(test.Valid)
			sidechainHeight++
			sidechain[sidechainHeight] = &p.Payload
		}
	}

	// Check the withdrawals on the latest
	ws.WithdrawalsHistory.VerifyWithdrawals(
		sidechainHeight,
		nil,
		t.TestEngine,
	)

	if ws.ReOrgViaSync {
		// Send latest sidechain payload as NewPayload + FCU and wait for sync
	loop:
		for {
			r := t.TestEngine.TestEngineNewPayloadV2(sidechain[sidechainHeight])
			r.ExpectNoError()
			p := t.TestEngine.TestEngineForkchoiceUpdatedV2(
				&beacon.ForkchoiceStateV1{
					HeadBlockHash: sidechain[sidechainHeight].BlockHash,
				},
				nil,
			)
			p.ExpectNoError()
			if p.Response.PayloadStatus.Status == test.Invalid {
				t.Fatalf("FAIL (%s): Primary client invalidated side chain", t.TestName)
			}
			select {
			case <-t.TimeoutContext.Done():
				t.Fatalf("FAIL (%s): Timeout waiting for sync", t.TestName)
			case <-time.After(time.Second):
				b := t.TestEngine.TestBlockByNumber(nil)
				if b.Block.Hash() == sidechain[sidechainHeight].BlockHash {
					// sync successful
					break loop
				}
			}
		}
	} else {
		// Send all payloads one by one to the primary client
		for payloadNumber := ws.GetSidechainSplitHeight(); payloadNumber <= sidechainHeight; payloadNumber++ {
			payload, ok := sidechain[payloadNumber]
			if !ok {
				t.Fatalf("FAIL (%s): Invalid payload %d requested.", t.TestName, payloadNumber)
			}
			var version int
			if payloadNumber >= ws.GetSidechainWithdrawalsForkHeight() {
				version = 2
			} else {
				version = 1
			}
			t.Logf("INFO (%s): Sending sidechain payload %d, hash=%s, parent=%s", t.TestName, payloadNumber, payload.BlockHash, payload.ParentHash)
			r := t.TestEngine.TestEngineNewPayload(payload, version)
			r.ExpectStatusEither(test.Valid, test.Accepted)
			p := t.TestEngine.TestEngineForkchoiceUpdated(
				&beacon.ForkchoiceStateV1{
					HeadBlockHash: payload.BlockHash,
				},
				nil,
				version,
			)
			p.ExpectPayloadStatus(test.Valid)
		}
	}

	// Verify withdrawals changed
	sidechainWithdrawalsHistory.VerifyWithdrawals(
		sidechainHeight,
		nil,
		t.TestEngine,
	)
	// Verify all balances of accounts in the original chain didn't increase
	// after the fork.
	// We are using different accounts credited between the canonical chain
	// and the fork.
	// We check on `latest`.
	ws.WithdrawalsHistory.VerifyWithdrawals(
		ws.WithdrawalsForkHeight-1,
		nil,
		t.TestEngine,
	)

	// Re-Org back to the canonical chain
	r := t.TestEngine.TestEngineForkchoiceUpdatedV2(&beacon.ForkchoiceStateV1{
		HeadBlockHash: t.CLMock.LatestPayloadBuilt.BlockHash,
	}, nil)
	r.ExpectPayloadStatus(test.Valid)
}
