// # Test suite for withdrawals tests
package suite_withdrawals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/beacon"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	client_types "github.com/ethereum/hive/simulators/ethereum/engine/client/types"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	Head               *big.Int // Nil
	Pending            = big.NewInt(-2)
	Finalized          = big.NewInt(-3)
	Safe               = big.NewInt(-4)
	InvalidParamsError = -32602
	MAX_INITCODE_SIZE  = 49152

	WARM_COINBASE_ADDRESS = common.HexToAddress("0x0101010101010101010101010101010101010101")
	PUSH0_ADDRESS         = common.HexToAddress("0x0202020202020202020202020202020202020202")

	TX_CONTRACT_ADDRESSES = []common.Address{
		WARM_COINBASE_ADDRESS,
		PUSH0_ADDRESS,
	}
)

// Execution specification reference:
// https://github.com/ethereum/execution-apis/blob/main/src/engine/specification.md

// List of all withdrawals tests
var Tests = []test.SpecInterface{
	&WithdrawalsBaseSpec{
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

	&WithdrawalsBaseSpec{
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

	&WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdrawals Fork on Block 2",
			About: `
			Tests the transition to the withdrawals fork after a single block
			has happened.
			Block 1 is sent with invalid non-null withdrawals payload and
			client is expected to respond with the appropriate error.
			`,
		},
		WithdrawalsForkHeight: 2, // Genesis and Block 1 are Pre-Withdrawals
		WithdrawalsBlockCount: 1,
		WithdrawalsPerBlock:   16,
	},

	&WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdrawals Fork on Block 3",
			About: `
			Tests the transition to the withdrawals fork after two blocks
			have happened.
			Block 2 is sent with invalid non-null withdrawals payload and
			client is expected to respond with the appropriate error.
			`,
		},
		WithdrawalsForkHeight: 3, // Genesis, Block 1 and 2 are Pre-Withdrawals
		WithdrawalsBlockCount: 1,
		WithdrawalsPerBlock:   16,
	},

	&WithdrawalsBaseSpec{
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

	&WithdrawalsBaseSpec{
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

	&WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdraw many accounts",
			About: `
			Make multiple withdrawals to 1024 different accounts.
			Execute many blocks this way.
			`,
			TimeoutSeconds: 240,
		},
		WithdrawalsForkHeight:    1,
		WithdrawalsBlockCount:    4,
		WithdrawalsPerBlock:      1024,
		WithdrawableAccountCount: 1024,
	},

	&WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Withdraw zero amount",
			About: `
			Make multiple withdrawals where the amount withdrawn is 0.
			`,
		},
		WithdrawalsForkHeight:    1,
		WithdrawalsBlockCount:    1,
		WithdrawalsPerBlock:      64,
		WithdrawableAccountCount: 2,
		WithdrawAmounts: []uint64{
			0,
			1,
		},
	},

	&WithdrawalsBaseSpec{
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

	&WithdrawalsBaseSpec{
		Spec: test.Spec{
			Name: "Corrupted Block Hash Payload (INVALID)",
			About: `
			Send a valid payload with a corrupted hash using engine_newPayloadV2.
			`,
		},
		WithdrawalsForkHeight:    1,
		WithdrawalsBlockCount:    1,
		TestCorrupedHashPayloads: true,
	},

	// Block value tests
	&BlockValueSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadV2 Block Value",
				About: `
				Verify the block value returned in GetPayloadV2.
				`,
			},
			WithdrawalsForkHeight: 1,
			WithdrawalsBlockCount: 1,
		},
	},

	// Sync Tests
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account - No Transactions",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 1
			- Withdraw to a single account 16 times each block for 2 blocks
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
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 1
			- Withdraw to a single account 16 times each block for 2 blocks
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
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Genesis - Single Withdrawal Account",
				About: `
			- Spawn a first client, with Withdrawals since genesis
			- Withdraw to a single account 16 times each block for 2 blocks
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
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts - No Transactions",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to 16 accounts each block for 2 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
			`,
			},
			WithdrawalsForkHeight:    2,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 16,
			TransactionsPerBlock:     common.Big0,
		},
		SyncSteps: 1,
	},
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to 16 accounts each block for 2 blocks
			- Spawn a secondary client and send FCUV2(head)
			- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
			`,
			},
			WithdrawalsForkHeight:    2,
			WithdrawalsBlockCount:    2,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 16,
		},
		SyncSteps: 1,
	},
	&WithdrawalsSyncSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts",
				About: `
			- Spawn a first client
			- Go through withdrawals fork on Block 2
			- Withdraw to many accounts 16 times each block for 128 blocks
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
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 1 Block Re-Org",
				About: `
				Tests a simple 1 block re-org 
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 1,
		ReOrgViaSync:    false,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload",
				About: `
				Tests a 8 block re-org using NewPayload
				Re-org does not change withdrawals fork height
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 8,
		ReOrgViaSync:    false,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org, Sync",
				About: `
				Tests a 8 block re-org using NewPayload
				Re-org does not change withdrawals fork height
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   16,
		},
		ReOrgBlockCount: 8,
		ReOrgViaSync:    true,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org NewPayload",
				About: `
				Tests a 10 block re-org using NewPayload
				Re-org does not change withdrawals fork height, but changes
				the payload at the height of the fork
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount: 10,
		ReOrgViaSync:    false,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Re-org does not change withdrawals fork height, but changes
				the payload at the height of the fork
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount: 10,
		ReOrgViaSync:    true,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org",
				About: `
				Tests a 10 block re-org using NewPayload
				Sidechain reaches withdrawals fork at a lower block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            false,
		SidechainTimeIncrements: 2,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Sidechain reaches withdrawals fork at a lower block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
			},
			WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
			WithdrawalsBlockCount: 8,
			WithdrawalsPerBlock:   128,
		},
		ReOrgBlockCount:         10,
		ReOrgViaSync:            true,
		SidechainTimeIncrements: 2,
	},
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org",
				About: `
				Tests a 10 block re-org using NewPayload
				Sidechain reaches withdrawals fork at a higher block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
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
	&WithdrawalsReorgSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync",
				About: `
				Tests a 10 block re-org using sync
				Sidechain reaches withdrawals fork at a higher block height
				than the canonical chain
				`,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
				TimeoutSeconds:   300,
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

	// EVM Tests (EIP-3651, EIP-3855, EIP-3860)
	&MaxInitcodeSizeSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "Max Initcode Size",
			},
			WithdrawalsForkHeight: 2, // Block 1 is Pre-Withdrawals
			WithdrawalsBlockCount: 2,
		},
		OverflowMaxInitcodeTxCountBeforeFork: 0,
		OverflowMaxInitcodeTxCountAfterFork:  1,
	},

	// Get Payload Bodies Requests
	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodiesByRange",
				About: `
				Make multiple withdrawals to 16 accounts each payload.
				Retrieve many of the payloads' bodies by number range.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight:    17,
			WithdrawalsBlockCount:    16,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 4,
			},
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 8,
			},
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 4,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 16,
				Count: 2,
			},
			GetPayloadBodyRequestByRange{
				Start: 17,
				Count: 16,
			},
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 32,
			},
			GetPayloadBodyRequestByRange{
				Start: 31,
				Count: 3,
			},
			GetPayloadBodyRequestByRange{
				Start: 32,
				Count: 2,
			},
			GetPayloadBodyRequestByRange{
				Start: 33,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 33,
				Count: 32,
			},
			GetPayloadBodyRequestByRange{
				Start: 32,
				Count: 0,
			},
			GetPayloadBodyRequestByRange{
				Start: 0,
				Count: 1,
			},
		},
	},

	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodies After Sync",
				About: `
				Make multiple withdrawals to 16 accounts each payload.
				Spawn a secondary client which must sync the canonical chain
				from the first client.
				Retrieve many of the payloads' bodies by number range from
				this secondary client.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight:    17,
			WithdrawalsBlockCount:    16,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByRange{
				Start: 16,
				Count: 2,
			},
			GetPayloadBodyRequestByRange{
				Start: 31,
				Count: 3,
			},
			GetPayloadBodyRequestByHashIndex{
				BlockNumbers: []uint64{
					1,
					16,
					2,
					17,
				},
			},
			GetPayloadBodyRequestByHashIndex{ // Existing+Random hashes
				BlockNumbers: []uint64{
					32,
					1000,
					31,
					1000,
					30,
					1000,
				},
			},
		},
	},

	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodiesByRange (Sidechain)",
				About: `
				Make multiple withdrawals to 16 accounts each payload.
				Retrieve many of the payloads' bodies by number range.
				Create a sidechain extending beyond the canonical chain block number.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight:    17,
			WithdrawalsBlockCount:    16,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		GenerateSidechain: true,
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByRange{
				Start: 33,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 32,
				Count: 2,
			},
		},
	},

	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodiesByRange (Empty Transactions/Withdrawals)",
				About: `
				Make no withdrawals and no transactions in many payloads.
				Retrieve many of the payloads' bodies by number range.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 2,
			WithdrawalsBlockCount: 1,
			WithdrawalsPerBlock:   0,
			TransactionsPerBlock:  common.Big0,
		},
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 2,
				Count: 1,
			},
			GetPayloadBodyRequestByRange{
				Start: 1,
				Count: 2,
			},
		},
	},
	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodiesByHash",
				About: `
				Make multiple withdrawals to 16 accounts each payload.
				Retrieve many of the payloads' bodies by hash.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight:    17,
			WithdrawalsBlockCount:    16,
			WithdrawalsPerBlock:      16,
			WithdrawableAccountCount: 1024,
		},
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByHashIndex{
				BlockNumbers: []uint64{
					1,
					16,
					2,
					17,
				},
			},
			GetPayloadBodyRequestByHashIndex{
				Start: 1,
				End:   32,
			},
			GetPayloadBodyRequestByHashIndex{ // Existing+Random hashes
				BlockNumbers: []uint64{
					32,
					1000,
					31,
					1000,
					30,
					1000,
				},
			},
			GetPayloadBodyRequestByHashIndex{ // All Random hashes
				BlockNumbers: []uint64{
					1000,
					1000,
					1000,
					1000,
					1000,
					1000,
				},
			},
		},
	},

	&GetPayloadBodiesSpec{
		WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
			Spec: test.Spec{
				Name: "GetPayloadBodiesByHash (Empty Transactions/Withdrawals)",
				About: `
				Make no withdrawals and no transactions in many payloads.
				Retrieve many of the payloads' bodies by hash.
				`,
				TimeoutSeconds:   240,
				SlotsToSafe:      big.NewInt(32),
				SlotsToFinalized: big.NewInt(64),
			},
			WithdrawalsForkHeight: 17,
			WithdrawalsBlockCount: 16,
			WithdrawalsPerBlock:   0,
			TransactionsPerBlock:  common.Big0,
		},
		GetPayloadBodiesRequests: []GetPayloadBodyRequest{
			GetPayloadBodyRequestByHashIndex{
				Start: 16,
				End:   17,
			},
		},
	},
}

// Helper types to convert gwei into wei more easily
func WeiAmount(w *types.Withdrawal) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(w.Amount), big.NewInt(1e9))
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
					balance.Add(balance, WeiAmount(withdrawal))
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
					currentBalance.Add(currentBalance, WeiAmount(withdrawal))
				} else {
					accounts[withdrawal.Address] = new(big.Int).Set(WeiAmount(withdrawal))
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
	WithdrawAmounts          []uint64           // Amounts of withdrawn wei on each withdrawal (round-robin)
	TransactionsPerBlock     *big.Int           // Amount of test transactions to include in withdrawal blocks
	TestCorrupedHashPayloads bool               // Send a valid payload with corrupted hash
	SkipBaseVerifications    bool               // For code reuse of the base spec procedure
}

// Get the per-block timestamp increments configured for this test
func (ws *WithdrawalsBaseSpec) GetBlockTimeIncrements() uint64 {
	if ws.TimeIncrements == 0 {
		return 1
	}
	return ws.TimeIncrements
}

// Timestamp delta between genesis and the withdrawals fork
func (ws *WithdrawalsBaseSpec) GetWithdrawalsGenesisTimeDelta() uint64 {
	return ws.WithdrawalsForkHeight * ws.GetBlockTimeIncrements()
}

// Calculates Shanghai fork timestamp given the amount of blocks that need to be
// produced beforehand.
func (ws *WithdrawalsBaseSpec) GetWithdrawalsForkTime() uint64 {
	return uint64(globals.GenesisTimestamp) + ws.GetWithdrawalsGenesisTimeDelta()
}

// Generates the fork config, including withdrawals fork timestamp.
func (ws *WithdrawalsBaseSpec) GetForkConfig() test.ForkConfig {
	return test.ForkConfig{
		ShanghaiTimestamp: big.NewInt(int64(ws.GetWithdrawalsForkTime())),
	}
}

// Get the start account for all withdrawals.
func (ws *WithdrawalsBaseSpec) GetWithdrawalsStartAccount() *big.Int {
	return big.NewInt(0x1000)
}

// Adds bytecode that unconditionally sets an storage key to specified account range
func AddUnconditionalBytecode(g *core.Genesis, start *big.Int, end *big.Int) {
	for ; start.Cmp(end) <= 0; start.Add(start, common.Big1) {
		accountAddress := common.BigToAddress(start)
		// Bytecode to unconditionally set a storage key
		g.Alloc[accountAddress] = core.GenesisAccount{
			Code: []byte{
				0x60, // PUSH1(0x01)
				0x01,
				0x60, // PUSH1(0x00)
				0x00,
				0x55, // SSTORE
				0x00, // STOP
			}, // sstore(0, 1)
			Nonce:   0,
			Balance: common.Big0,
		}
	}
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (ws *WithdrawalsBaseSpec) GetGenesis() *core.Genesis {
	genesis := ws.Spec.GetGenesis()

	// Remove PoW altogether
	genesis.Difficulty = common.Big0
	genesis.Config.TerminalTotalDifficulty = common.Big0
	genesis.Config.Clique = nil
	genesis.ExtraData = []byte{}

	// Add some accounts to withdraw to with unconditional SSTOREs
	startAccount := big.NewInt(0x1000)
	endAccount := big.NewInt(0x1000 + int64(ws.GetWithdrawableAccountCount()) - 1)
	AddUnconditionalBytecode(genesis, startAccount, endAccount)

	// Add accounts that use the coinbase (EIP-3651)
	warmCoinbaseCode := []byte{
		0x5A, // GAS
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x60, // PUSH1(0x00)
		0x00,
		0x41, // COINBASE
		0x60, // PUSH1(0xFF)
		0xFF,
		0xF1, // CALL
		0x5A, // GAS
		0x90, // SWAP1
		0x50, // POP - Call result
		0x90, // SWAP1
		0x03, // SUB
		0x60, // PUSH1(0x16) - GAS + PUSH * 6 + COINBASE
		0x16,
		0x90, // SWAP1
		0x03, // SUB
		0x43, // NUMBER
		0x55, // SSTORE
	}
	genesis.Alloc[WARM_COINBASE_ADDRESS] = core.GenesisAccount{
		Code:    warmCoinbaseCode,
		Balance: common.Big0,
	}

	// Add accounts that use the PUSH0 (EIP-3855)
	push0Code := []byte{
		0x43, // NUMBER
		0x5F, // PUSH0
		0x55, // SSTORE
	}
	genesis.Alloc[PUSH0_ADDRESS] = core.GenesisAccount{
		Code:    push0Code,
		Balance: common.Big0,
	}
	return genesis
}

func (ws *WithdrawalsBaseSpec) VerifyContractsStorage(t *test.Env) {
	if ws.GetTransactionCountPerPayload() < uint64(len(TX_CONTRACT_ADDRESSES)) {
		return
	}
	// Assume that forkchoice updated has been already sent
	latestPayloadNumber := t.CLMock.LatestExecutedPayload.Number
	latestPayloadNumberBig := big.NewInt(int64(latestPayloadNumber))

	r := t.TestEngine.TestStorageAt(WARM_COINBASE_ADDRESS, common.BigToHash(latestPayloadNumberBig), latestPayloadNumberBig)
	p := t.TestEngine.TestStorageAt(PUSH0_ADDRESS, common.Hash{}, latestPayloadNumberBig)
	if latestPayloadNumber >= ws.WithdrawalsForkHeight {
		// Shanghai
		r.ExpectBigIntStorageEqual(big.NewInt(100))        // WARM_STORAGE_READ_COST
		p.ExpectBigIntStorageEqual(latestPayloadNumberBig) // tx succeeded
	} else {
		// Pre-Shanghai
		r.ExpectBigIntStorageEqual(big.NewInt(2600)) // COLD_ACCOUNT_ACCESS_COST
		p.ExpectBigIntStorageEqual(big.NewInt(0))    // tx must've failed
	}
}

// Changes the CL Mocker default time increments of 1 to the value specified
// in the test spec.
func (ws *WithdrawalsBaseSpec) ConfigureCLMock(cl *clmock.CLMocker) {
	cl.BlockTimestampIncrement = big.NewInt(int64(ws.GetBlockTimeIncrements()))
}

// Number of blocks to be produced (not counting genesis) before withdrawals
// fork.
func (ws *WithdrawalsBaseSpec) GetPreWithdrawalsBlockCount() uint64 {
	if ws.WithdrawalsForkHeight == 0 {
		return 0
	} else {
		return ws.WithdrawalsForkHeight - 1
	}
}

// Number of payloads to be produced (pre and post withdrawals) during the entire test
func (ws *WithdrawalsBaseSpec) GetTotalPayloadCount() uint64 {
	return ws.GetPreWithdrawalsBlockCount() + ws.WithdrawalsBlockCount
}

func (ws *WithdrawalsBaseSpec) GetWithdrawableAccountCount() uint64 {
	if ws.WithdrawableAccountCount == 0 {
		// Withdraw to 16 accounts by default
		return 16
	}
	return ws.WithdrawableAccountCount
}

// Generates a list of withdrawals based on current configuration
func (ws *WithdrawalsBaseSpec) GenerateWithdrawalsForBlock(nextIndex uint64, startAccount *big.Int) (types.Withdrawals, uint64) {
	differentAccounts := ws.GetWithdrawableAccountCount()
	withdrawAmounts := ws.WithdrawAmounts
	if withdrawAmounts == nil {
		withdrawAmounts = []uint64{
			1,
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

func (ws *WithdrawalsBaseSpec) GetTransactionCountPerPayload() uint64 {
	if ws.TransactionsPerBlock == nil {
		return 16
	}
	return ws.TransactionsPerBlock.Uint64()
}

// Base test case execution procedure for withdrawals
func (ws *WithdrawalsBaseSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)

	t.CLMock.WaitForTTD()

	// Check if we have pre-Shanghai blocks
	if ws.GetWithdrawalsForkTime() > uint64(globals.GenesisTimestamp) {
		// Check `latest` during all pre-shanghai blocks, none should
		// contain `withdrawalsRoot`, including genesis.

		// Genesis should not contain `withdrawalsRoot` either
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectationDescription = `
		Requested "latest" block expecting genesis to contain
		withdrawalRoot=nil, because genesis.timestamp < shanghaiTime
		`
		r.ExpectWithdrawalsRoot(nil)
	} else {
		// Genesis is post shanghai, it should contain EmptyWithdrawalsRoot
		r := t.TestEngine.TestBlockByNumber(nil)
		r.ExpectationDescription = `
		Requested "latest" block expecting genesis to contain
		withdrawalRoot=EmptyTrieRoot, because genesis.timestamp >= shanghaiTime
		`
		r.ExpectWithdrawalsRoot(helper.EmptyWithdrawalsRootHash)
	}

	// Produce any blocks necessary to reach withdrawals fork
	t.CLMock.ProduceBlocks(int(ws.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {

			// Send some transactions
			for i := uint64(0); i < ws.GetTransactionCountPerPayload(); i++ {

				var destAddr = TX_CONTRACT_ADDRESSES[int(i)%len(TX_CONTRACT_ADDRESSES)]

				_, err := helper.SendNextTransaction(
					t.TestContext,
					t.CLMock.NextBlockProducer,
					&helper.BaseTransactionCreator{
						Recipient: &destAddr,
						Amount:    common.Big1,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)

				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			}

			if !ws.SkipBaseVerifications {
				// Try to send a ForkchoiceUpdatedV2 with non-null
				// withdrawals before Shanghai
				r := t.TestEngine.TestEngineForkchoiceUpdatedV2(
					&beacon.ForkchoiceStateV1{
						HeadBlockHash: t.CLMock.LatestHeader.Hash(),
					},
					&beacon.PayloadAttributes{
						Timestamp:             t.CLMock.LatestHeader.Time + ws.GetBlockTimeIncrements(),
						Random:                common.Hash{},
						SuggestedFeeRecipient: common.Address{},
						Withdrawals:           make(types.Withdrawals, 0),
					},
				)
				r.ExpectationDescription = "Sent pre-shanghai Forkchoice using ForkchoiceUpdatedV2 + Withdrawals, error is expected"
				r.ExpectErrorCode(InvalidParamsError)

				// Send a valid Pre-Shanghai request using ForkchoiceUpdatedV2
				// (CLMock uses V1 by default)
				r = t.TestEngine.TestEngineForkchoiceUpdatedV2(
					&beacon.ForkchoiceStateV1{
						HeadBlockHash: t.CLMock.LatestHeader.Hash(),
					},
					&beacon.PayloadAttributes{
						Timestamp:             t.CLMock.LatestHeader.Time + ws.GetBlockTimeIncrements(),
						Random:                common.Hash{},
						SuggestedFeeRecipient: common.Address{},
						Withdrawals:           nil,
					},
				)
				r.ExpectationDescription = "Sent pre-shanghai Forkchoice ForkchoiceUpdatedV2 + null withdrawals, no error is expected"
				r.ExpectNoError()
			}
		},
		OnGetPayload: func() {
			if !ws.SkipBaseVerifications {
				// Try to get the same payload but use `engine_getPayloadV2`
				g := t.TestEngine.TestEngineGetPayloadV2(t.CLMock.NextPayloadID)
				g.ExpectPayload(&t.CLMock.LatestPayloadBuilt)

				// Send produced payload but try to include non-nil
				// `withdrawals`, it should fail.
				emptyWithdrawalsList := make(types.Withdrawals, 0)
				payloadPlusWithdrawals, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					Withdrawals: emptyWithdrawalsList,
				})
				if err != nil {
					t.Fatalf("Unable to append withdrawals: %v", err)
				}
				r := t.TestEngine.TestEngineNewPayloadV2(payloadPlusWithdrawals)
				r.ExpectationDescription = "Sent pre-shanghai payload using NewPayloadV2+Withdrawals, error is expected"
				r.ExpectErrorCode(InvalidParamsError)

				// Send valid ExecutionPayloadV1 using engine_newPayloadV2
				r = t.TestEngine.TestEngineNewPayloadV2(&t.CLMock.LatestPayloadBuilt)
				r.ExpectationDescription = "Sent pre-shanghai payload using NewPayloadV2, no error is expected"
				r.ExpectStatus(test.Valid)
			}
		},
		OnNewPayloadBroadcast: func() {
			if !ws.SkipBaseVerifications {
				// We sent a pre-shanghai FCU.
				// Keep expecting `nil` until Shanghai.
				r := t.TestEngine.TestBlockByNumber(nil)
				r.ExpectationDescription = fmt.Sprintf(`
				Requested "latest" block expecting block to contain
				withdrawalRoot=nil, because (block %d).timestamp < shanghaiTime
				`,
					t.CLMock.LatestPayloadBuilt.Number,
				)
				r.ExpectWithdrawalsRoot(nil)
			}
		},
		OnForkchoiceBroadcast: func() {
			if !ws.SkipBaseVerifications {
				ws.VerifyContractsStorage(t)
			}
		},
	})

	// Produce requested post-shanghai blocks
	// (At least 1 block will be produced after this procedure ends).
	var (
		startAccount = ws.GetWithdrawalsStartAccount()
		nextIndex    = uint64(0)
	)

	t.CLMock.ProduceBlocks(int(ws.WithdrawalsBlockCount), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {

			if !ws.SkipBaseVerifications {
				// Try to send a PayloadAttributesV1 with null withdrawals after
				// Shanghai
				r := t.TestEngine.TestEngineForkchoiceUpdatedV2(
					&beacon.ForkchoiceStateV1{
						HeadBlockHash: t.CLMock.LatestHeader.Hash(),
					},
					&beacon.PayloadAttributes{
						Timestamp:             t.CLMock.LatestHeader.Time + ws.GetBlockTimeIncrements(),
						Random:                common.Hash{},
						SuggestedFeeRecipient: common.Address{},
						Withdrawals:           nil,
					},
				)
				r.ExpectationDescription = "Sent shanghai fcu using PayloadAttributesV1, error is expected"
				r.ExpectErrorCode(InvalidParamsError)
			}

			// Send some withdrawals
			t.CLMock.NextWithdrawals, nextIndex = ws.GenerateWithdrawalsForBlock(nextIndex, startAccount)
			ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
			// Send some transactions
			for i := uint64(0); i < ws.GetTransactionCountPerPayload(); i++ {
				var destAddr = TX_CONTRACT_ADDRESSES[int(i)%len(TX_CONTRACT_ADDRESSES)]

				_, err := helper.SendNextTransaction(
					t.TestContext,
					t.CLMock.NextBlockProducer,
					&helper.BaseTransactionCreator{
						Recipient: &destAddr,
						Amount:    common.Big1,
						Payload:   nil,
						TxType:    t.TestTransactionType,
						GasLimit:  75000,
					},
				)

				if err != nil {
					t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
				}
			}
		},
		OnGetPayload: func() {
			if !ws.SkipBaseVerifications {
				// Send invalid `ExecutionPayloadV1` by replacing withdrawals list
				// with null, and client must respond with `InvalidParamsError`.
				// Note that StateRoot is also incorrect but null withdrawals should
				// be checked first instead of responding `INVALID`
				nilWithdrawalsPayload, err := helper.CustomizePayload(&t.CLMock.LatestPayloadBuilt, &helper.CustomPayloadData{
					RemoveWithdrawals: true,
				})
				if err != nil {
					t.Fatalf("Unable to append withdrawals: %v", err)
				}
				r := t.TestEngine.TestEngineNewPayloadV2(nilWithdrawalsPayload)
				r.ExpectationDescription = "Sent shanghai payload using ExecutionPayloadV1, error is expected"
				r.ExpectErrorCode(InvalidParamsError)

				// Verify the list of withdrawals returned on the payload built
				// completely matches the list provided in the
				// engine_forkchoiceUpdatedV2 method call
				if sentList, ok := ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber]; !ok {
					panic("withdrawals sent list was not saved")
				} else {
					if len(sentList) != len(t.CLMock.LatestPayloadBuilt.Withdrawals) {
						t.Fatalf("FAIL (%s): Incorrect list of withdrawals on built payload: want=%d, got=%d", t.TestName, len(sentList), len(t.CLMock.LatestPayloadBuilt.Withdrawals))
					}
					for i := 0; i < len(sentList); i++ {
						if err := test.CompareWithdrawal(sentList[i], t.CLMock.LatestPayloadBuilt.Withdrawals[i]); err != nil {
							t.Fatalf("FAIL (%s): Incorrect withdrawal on index %d: %v", t.TestName, i, err)
						}
					}

				}
			}
		},
		OnNewPayloadBroadcast: func() {
			// Check withdrawal addresses and verify withdrawal balances
			// have not yet been applied
			if !ws.SkipBaseVerifications {
				for _, addr := range ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
					// Test balance at `latest`, which should not yet have the
					// withdrawal applied.
					expectedAccountBalance := ws.WithdrawalsHistory.GetExpectedAccountBalance(
						addr,
						t.CLMock.LatestExecutedPayload.Number-1)
					r := t.TestEngine.TestBalanceAt(addr, nil)
					r.ExpectationDescription = fmt.Sprintf(`
						Requested balance for account %s on "latest" block
						after engine_newPayloadV2, expecting balance to be equal
						to value on previous block (%d), since the new payload
						has not yet been applied.
						`,
						addr,
						t.CLMock.LatestExecutedPayload.Number-1,
					)
					r.ExpectBalanceEqual(expectedAccountBalance)
				}

				if ws.TestCorrupedHashPayloads {
					payload := t.CLMock.LatestExecutedPayload

					// Corrupt the hash
					rand.Read(payload.BlockHash[:])

					// On engine_newPayloadV2 `INVALID_BLOCK_HASH` is deprecated
					// in favor of reusing `INVALID`
					n := t.TestEngine.TestEngineNewPayloadV2(&payload)
					n.ExpectStatus(test.Invalid)
				}
			}
		},
		OnForkchoiceBroadcast: func() {
			// Check withdrawal addresses and verify withdrawal balances
			// have been applied
			if !ws.SkipBaseVerifications {
				for _, addr := range ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
					// Test balance at `latest`, which should have the
					// withdrawal applied.
					r := t.TestEngine.TestBalanceAt(addr, nil)
					r.ExpectationDescription = fmt.Sprintf(`
						Requested balance for account %s on "latest" block
						after engine_forkchoiceUpdatedV2, expecting balance to
						be equal to value on latest payload (%d), since the new payload
						has not yet been applied.
						`,
						addr,
						t.CLMock.LatestExecutedPayload.Number,
					)
					r.ExpectBalanceEqual(
						ws.WithdrawalsHistory.GetExpectedAccountBalance(
							addr,
							t.CLMock.LatestExecutedPayload.Number),
					)
				}
				// Check the correct withdrawal root on `latest` block
				r := t.TestEngine.TestBlockByNumber(nil)
				expectedWithdrawalsRoot := helper.ComputeWithdrawalsRoot(
					ws.WithdrawalsHistory.GetWithdrawals(
						t.CLMock.LatestExecutedPayload.Number,
					),
				)
				jsWithdrawals, _ := json.MarshalIndent(ws.WithdrawalsHistory.GetWithdrawals(t.CLMock.LatestExecutedPayload.Number), "", " ")
				r.ExpectationDescription = fmt.Sprintf(`
						Requested "latest" block after engine_forkchoiceUpdatedV2,
						to verify withdrawalsRoot with the following withdrawals:
						%s`, jsWithdrawals)
				r.ExpectWithdrawalsRoot(&expectedWithdrawalsRoot)

				ws.VerifyContractsStorage(t)
			}
		},
	})
	// Iterate over balance history of withdrawn accounts using RPC and
	// check that the balances match expected values.
	// Also check one block before the withdrawal took place, verify that
	// withdrawal has not been updated.
	if !ws.SkipBaseVerifications {
		for block := uint64(0); block <= t.CLMock.LatestExecutedPayload.Number; block++ {
			ws.WithdrawalsHistory.VerifyWithdrawals(block, big.NewInt(int64(block)), t.TestEngine)

			// Check the correct withdrawal root on past blocks
			r := t.TestEngine.TestBlockByNumber(big.NewInt(int64(block)))
			var expectedWithdrawalsRoot *common.Hash = nil
			if block >= ws.WithdrawalsForkHeight {
				calcWithdrawalsRoot := helper.ComputeWithdrawalsRoot(
					ws.WithdrawalsHistory.GetWithdrawals(block),
				)
				expectedWithdrawalsRoot = &calcWithdrawalsRoot
			}
			jsWithdrawals, _ := json.MarshalIndent(ws.WithdrawalsHistory.GetWithdrawals(block), "", " ")
			r.ExpectationDescription = fmt.Sprintf(`
						Requested block %d to verify withdrawalsRoot with the
						following withdrawals:
						%s`, block, jsWithdrawals)

			r.ExpectWithdrawalsRoot(expectedWithdrawalsRoot)

		}

		// Verify on `latest`
		ws.WithdrawalsHistory.VerifyWithdrawals(t.CLMock.LatestExecutedPayload.Number, nil, t.TestEngine)
	}
}

// Withdrawals sync spec:
// Specifies a withdrawals test where the withdrawals happen and then a
// client needs to sync and apply the withdrawals.
type WithdrawalsSyncSpec struct {
	*WithdrawalsBaseSpec
	SyncSteps      int  // Sync block chunks that will be passed as head through FCUs to the syncing client
	SyncShouldFail bool //
}

func (ws *WithdrawalsSyncSpec) Execute(t *test.Env) {
	// Do the base withdrawal test first, skipping base verifications
	ws.WithdrawalsBaseSpec.SkipBaseVerifications = true
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
	}
	ws.WithdrawalsHistory.VerifyWithdrawals(t.CLMock.LatestHeader.Number.Uint64(), nil, secondaryEngineTest)
}

// Withdrawals re-org spec:
// Specifies a withdrawals test where the withdrawals re-org can happen
// even to a point before withdrawals were enabled, or simply to a previous
// withdrawals block.
type WithdrawalsReorgSpec struct {
	*WithdrawalsBaseSpec

	ReOrgBlockCount         uint64 // How many blocks the re-org will replace, including the head
	ReOrgViaSync            bool   // Whether the client should fetch the sidechain by syncing from the secondary client
	SidechainTimeIncrements uint64
}

func (ws *WithdrawalsReorgSpec) GetSidechainSplitHeight() uint64 {
	if ws.ReOrgBlockCount > ws.GetTotalPayloadCount() {
		panic("invalid payload/re-org configuration")
	}
	return ws.GetTotalPayloadCount() + 1 - ws.ReOrgBlockCount
}

func (ws *WithdrawalsReorgSpec) GetSidechainBlockTimeIncrements() uint64 {
	if ws.SidechainTimeIncrements == 0 {
		return ws.GetBlockTimeIncrements()
	}
	return ws.SidechainTimeIncrements
}

func (ws *WithdrawalsReorgSpec) GetSidechainWithdrawalsForkHeight() uint64 {
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

func (ws *WithdrawalsReorgSpec) Execute(t *test.Env) {
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
			txs, err := helper.SendNextTransactions(
				t.TestContext,
				t.CLMock.NextBlockProducer,
				&helper.BaseTransactionCreator{
					Recipient: &globals.PrevRandaoContractAddr,
					Amount:    common.Big1,
					Payload:   nil,
					TxType:    t.TestTransactionType,
					GasLimit:  75000,
				},
				ws.GetTransactionCountPerPayload(),
			)
			if err != nil {
				t.Fatalf("FAIL (%s): Error trying to send transactions: %v", t.TestName, err)
			}

			// Error will be ignored here since the tx could have been already relayed
			secondaryEngine.SendTransactions(t.TestContext, txs)

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

// EIP-3860 Shanghai Tests:
// Send transactions overflowing the MAX_INITCODE_SIZE
// limit set in EIP-3860, before and after the Shanghai
// fork.
type MaxInitcodeSizeSpec struct {
	*WithdrawalsBaseSpec
	OverflowMaxInitcodeTxCountBeforeFork uint64
	OverflowMaxInitcodeTxCountAfterFork  uint64
}

func (s *MaxInitcodeSizeSpec) Execute(t *test.Env) {
	t.CLMock.WaitForTTD()

	invalidTxCreator := &helper.BigInitcodeTransactionCreator{
		InitcodeLength: MAX_INITCODE_SIZE + 1,
		BaseTransactionCreator: helper.BaseTransactionCreator{
			GasLimit: 2000000,
		},
	}
	validTxCreator := &helper.BigInitcodeTransactionCreator{
		InitcodeLength: MAX_INITCODE_SIZE,
		BaseTransactionCreator: helper.BaseTransactionCreator{
			GasLimit: 2000000,
		},
	}

	if s.OverflowMaxInitcodeTxCountBeforeFork > 0 {
		if s.GetPreWithdrawalsBlockCount() == 0 {
			panic("invalid test configuration")
		}

		for i := uint64(0); i < s.OverflowMaxInitcodeTxCountBeforeFork; i++ {
			tx, err := invalidTxCreator.MakeTransaction(i)
			if err != nil {
				t.Fatalf("FAIL: Error creating max initcode transaction: %v", err)
			}
			err = t.Engine.SendTransaction(t.TestContext, tx)
			if err != nil {
				t.Fatalf("FAIL: Error sending max initcode transaction before Shanghai: %v", err)
			}
		}
	}

	// Produce all blocks needed to reach Shanghai
	t.Logf("INFO: Blocks until Shanghai=%d", s.GetPreWithdrawalsBlockCount())
	txIncluded := uint64(0)
	t.CLMock.ProduceBlocks(int(s.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			t.Logf("INFO: Got Pre-Shanghai block=%d", t.CLMock.LatestPayloadBuilt.Number)
			txIncluded += uint64(len(t.CLMock.LatestPayloadBuilt.Transactions))
		},
	})

	// Check how many transactions were included
	if txIncluded == 0 && s.OverflowMaxInitcodeTxCountBeforeFork > 0 {
		t.Fatalf("FAIL: No max initcode txs included before Shanghai. Txs must have been included before the MAX_INITCODE_SIZE limit was enabled")
	}

	// Create a payload, no txs should be included
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnGetPayload: func() {
			if len(t.CLMock.LatestPayloadBuilt.Transactions) > 0 {
				t.Fatalf("FAIL: Client included tx exceeding the MAX_INITCODE_SIZE in payload")
			}
		},
	})

	// Send transactions after the fork
	for i := txIncluded; i < (txIncluded + s.OverflowMaxInitcodeTxCountAfterFork); i++ {
		tx, err := invalidTxCreator.MakeTransaction(i)
		if err != nil {
			t.Fatalf("FAIL: Error creating max initcode transaction: %v", err)
		}
		err = t.Engine.SendTransaction(t.TestContext, tx)
		if err == nil {
			t.Fatalf("FAIL: Client accepted tx exceeding the MAX_INITCODE_SIZE: %v", tx)
		}
		txBack, isPending, err := t.Engine.TransactionByHash(t.TestContext, tx.Hash())
		if txBack != nil || isPending || err == nil {
			t.Fatalf("FAIL: Invalid tx was not unknown to the client: txBack=%v, isPending=%t, err=%v", txBack, isPending, err)
		}
	}

	// Try to include an invalid tx in new payload
	var (
		validTx, _   = validTxCreator.MakeTransaction(txIncluded)
		invalidTx, _ = invalidTxCreator.MakeTransaction(txIncluded)
	)
	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			t.Engine.SendTransaction(t.TestContext, validTx)
		},
		OnGetPayload: func() {
			validTxBytes, err := validTx.MarshalBinary()
			if err != nil {
				t.Fatalf("FAIL: Unable to marshal valid tx to binary: %v", err)
			}
			if len(t.CLMock.LatestPayloadBuilt.Transactions) != 1 || !bytes.Equal(validTxBytes, t.CLMock.LatestPayloadBuilt.Transactions[0]) {
				t.Fatalf("FAIL: Client did not include valid tx with MAX_INITCODE_SIZE")
			}
			// Customize the payload to include a tx with an invalid initcode
			customPayload, err := helper.CustomizePayloadTransactions(&t.CLMock.LatestPayloadBuilt, types.Transactions{invalidTx})
			if err != nil {
				t.Fatalf("FAIL: Unable to customize payload: %v", err)
			}

			r := t.TestEngine.TestEngineNewPayloadV2(customPayload)
			r.ExpectStatus(test.Invalid)
			r.ExpectLatestValidHash(&t.CLMock.LatestPayloadBuilt.ParentHash)
		},
	})
}

// Withdrawals sync spec:
// Specifies a withdrawals test where the withdrawals happen and then a
// client needs to sync and apply the withdrawals.
type GetPayloadBodiesSpec struct {
	*WithdrawalsBaseSpec
	GetPayloadBodiesRequests []GetPayloadBodyRequest
	GenerateSidechain        bool
	AfterSync                bool
}

type GetPayloadBodyRequest interface {
	Verify(*test.TestEngineClient, clmock.ExecutableDataHistory)
}

type GetPayloadBodyRequestByRange struct {
	Start uint64
	Count uint64
}

func (req GetPayloadBodyRequestByRange) Verify(testEngine *test.TestEngineClient, payloadHistory clmock.ExecutableDataHistory) {
	r := testEngine.TestEngineGetPayloadBodiesByRangeV1(req.Start, req.Count)
	if req.Start < 1 || req.Count < 1 {
		r.ExpectationDescription = fmt.Sprintf(`
			Sent start (%d) or count (%d) to engine_getPayloadBodiesByRangeV1 with a
			value less than 1, therefore error is expected.
			`, req.Start, req.Count)
		r.ExpectErrorCode(InvalidParamsError)
		return
	}
	latestPayloadNumber := payloadHistory.LatestPayloadNumber()
	if req.Start > latestPayloadNumber {
		r.ExpectationDescription = fmt.Sprintf(`
			Sent start=%d and count=%d to engine_getPayloadBodiesByRangeV1, latest known block is %d, hence an empty list is expected.
			`, req.Start, req.Count, latestPayloadNumber)
		r.ExpectPayloadBodiesCount(0)
	} else {
		var count = req.Count
		if req.Start+req.Count-1 > latestPayloadNumber {
			count = latestPayloadNumber - req.Start + 1
		}
		r.ExpectationDescription = fmt.Sprintf("Sent engine_getPayloadBodiesByRange(start=%d, count=%d), latest payload number in canonical chain is %d", req.Start, req.Count, latestPayloadNumber)
		r.ExpectPayloadBodiesCount(count)
		for i := req.Start; i < req.Start+count; i++ {
			p := payloadHistory[i]

			r.ExpectPayloadBody(i-req.Start, &client_types.ExecutionPayloadBodyV1{
				Transactions: p.Transactions,
				Withdrawals:  p.Withdrawals,
			})
		}
	}
}

type GetPayloadBodyRequestByHashIndex struct {
	BlockNumbers []uint64
	Start        uint64
	End          uint64
}

func (req GetPayloadBodyRequestByHashIndex) Verify(testEngine *test.TestEngineClient, payloadHistory clmock.ExecutableDataHistory) {
	payloads := make([]*beacon.ExecutableData, 0)
	hashes := make([]common.Hash, 0)
	if len(req.BlockNumbers) > 0 {
		for _, n := range req.BlockNumbers {
			if p, ok := payloadHistory[n]; ok {
				payloads = append(payloads, p)
				hashes = append(hashes, p.BlockHash)
			} else {
				// signal to request an unknown hash (random)
				randHash := common.Hash{}
				rand.Read(randHash[:])
				payloads = append(payloads, nil)
				hashes = append(hashes, randHash)
			}
		}
	}
	if req.Start > 0 && req.End > 0 {
		for n := req.Start; n <= req.End; n++ {
			if p, ok := payloadHistory[n]; ok {
				payloads = append(payloads, p)
				hashes = append(hashes, p.BlockHash)
			} else {
				// signal to request an unknown hash (random)
				randHash := common.Hash{}
				rand.Read(randHash[:])
				payloads = append(payloads, nil)
				hashes = append(hashes, randHash)
			}
		}
	}
	if len(payloads) == 0 {
		panic("invalid test")
	}

	r := testEngine.TestEngineGetPayloadBodiesByHashV1(hashes)
	r.ExpectPayloadBodiesCount(uint64(len(payloads)))
	for i, p := range payloads {
		var expectedPayloadBody *client_types.ExecutionPayloadBodyV1
		if p != nil {
			expectedPayloadBody = &client_types.ExecutionPayloadBodyV1{
				Transactions: p.Transactions,
				Withdrawals:  p.Withdrawals,
			}
		}
		r.ExpectPayloadBody(uint64(i), expectedPayloadBody)
	}

}

func (ws *GetPayloadBodiesSpec) Execute(t *test.Env) {
	// Do the base withdrawal test first, skipping base verifications
	ws.WithdrawalsBaseSpec.SkipBaseVerifications = true
	ws.WithdrawalsBaseSpec.Execute(t)

	payloadHistory := t.CLMock.ExecutedPayloadHistory

	testEngine := t.TestEngine

	if ws.GenerateSidechain {

		// First generate an extra payload on top of the canonical chain
		// Generate more withdrawals
		nextWithdrawals, _ := ws.GenerateWithdrawalsForBlock(payloadHistory.LatestWithdrawalsIndex(), ws.GetWithdrawalsStartAccount())

		f := t.TestEngine.TestEngineForkchoiceUpdatedV2(
			&beacon.ForkchoiceStateV1{
				HeadBlockHash: t.CLMock.LatestHeader.Hash(),
			},
			&beacon.PayloadAttributes{
				Timestamp:   t.CLMock.LatestHeader.Time + ws.GetBlockTimeIncrements(),
				Withdrawals: nextWithdrawals,
			},
		)
		f.ExpectPayloadStatus(test.Valid)

		// Wait for payload to be built
		time.Sleep(time.Second)

		// Get the next canonical payload
		p := t.TestEngine.TestEngineGetPayloadV2(f.Response.PayloadID)
		p.ExpectNoError()
		nextCanonicalPayload := &p.Payload

		// Now we have an extra payload that follows the canonical chain,
		// but we need a side chain for the test.
		sidechainCurrent, err := helper.CustomizePayload(&t.CLMock.LatestExecutedPayload, &helper.CustomPayloadData{
			Withdrawals: helper.RandomizeWithdrawalsOrder(t.CLMock.LatestExecutedPayload.Withdrawals),
		})
		if err != nil {
			t.Fatalf("FAIL (%s): Error obtaining custom sidechain payload: %v", t.TestName, err)
		}

		sidechainHead, err := helper.CustomizePayload(nextCanonicalPayload, &helper.CustomPayloadData{
			ParentHash:  &sidechainCurrent.BlockHash,
			Withdrawals: helper.RandomizeWithdrawalsOrder(nextCanonicalPayload.Withdrawals),
		})
		if err != nil {
			t.Fatalf("FAIL (%s): Error obtaining custom sidechain payload: %v", t.TestName, err)
		}

		// Send both sidechain payloads as engine_newPayloadV2
		n1 := t.TestEngine.TestEngineNewPayloadV2(sidechainCurrent)
		n1.ExpectStatus(test.Valid)
		n2 := t.TestEngine.TestEngineNewPayloadV2(sidechainHead)
		n2.ExpectStatus(test.Valid)
	} else if ws.AfterSync {
		// Spawn a secondary client which will need to sync to the primary client
		secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
		if err != nil {
			t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
		}
		secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
		t.CLMock.AddEngineClient(secondaryEngine)

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

		// GetPayloadBodies will be sent to the secondary client
		testEngine = secondaryEngineTest
	}

	// Now send the range request, which should ignore any sidechain
	for _, req := range ws.GetPayloadBodiesRequests {
		req.Verify(testEngine, payloadHistory)
	}
}

type BlockValueSpec struct {
	*WithdrawalsBaseSpec
}

func (s *BlockValueSpec) Execute(t *test.Env) {
	s.WithdrawalsBaseSpec.SkipBaseVerifications = true
	s.WithdrawalsBaseSpec.Execute(t)

	// Get the latest block and the transactions included
	b := t.TestEngine.TestBlockByNumber(nil)
	b.ExpectNoError()

	totalValue := new(big.Int)
	txs := b.Block.Transactions()
	if len(txs) == 0 {
		t.Fatalf("FAIL (%s): No transactions included in latest block", t.TestName)
	}
	for _, tx := range txs {
		r := t.TestEngine.TestTransactionReceipt(tx.Hash())
		r.ExpectNoError()

		receipt := r.Receipt

		gasUsed := new(big.Int).SetUint64(receipt.GasUsed)
		txTip, _ := tx.EffectiveGasTip(b.Block.Header().BaseFee)
		txTip.Mul(txTip, gasUsed)
		totalValue.Add(totalValue, txTip)
	}

	if totalValue.Cmp(t.CLMock.LatestBlockValue) != 0 {
		t.Fatalf("FAIL (%s): Unexpected block value returned on GetPayloadV2: want=%d, got=%d", t.TestName, totalValue, t.CLMock.LatestBlockValue)
	}
}
