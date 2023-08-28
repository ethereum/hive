// # Test suite for withdrawals tests
package suite_withdrawals

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	beacon "github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/hive/simulators/ethereum/engine/client/hive_rpc"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/libgno"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

var (
	Head               *big.Int // Nil
	Pending            = big.NewInt(-2)
	Finalized          = big.NewInt(-3)
	Safe               = big.NewInt(-4)
	InvalidParamsError = -32602
	MAX_INITCODE_SIZE  = 49152

	/*
		Warm coinbase contract needs to check if EIP-3651 applied after shapella
		https://eips.ethereum.org/EIPS/eip-3651

		Contract bytecode saves coinbase access cost to the slot number of current block
		i.e. if current block number is 5 ==> coinbase access cost saved to slot 5 etc
	*/
	WARM_COINBASE_ADDRESS = common.HexToAddress("0x0101010101010101010101010101010101010101")
	warmCoinbaseCode      = []byte{
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
	/*
		PUSH0 contract needs to check if EIP-3855 applied after shapella
		https://eips.ethereum.org/EIPS/eip-3855

		Contract bytecode reverts tx before the shapells (because PUSH0 opcode does not exists)
		After shapella hardfork it saves current block number to 0 slot
	*/
	PUSH0_ADDRESS = common.HexToAddress("0x0202020202020202020202020202020202020202")
	push0Code     = []byte{
		0x43, // NUMBER
		0x5F, // PUSH0
		0x55, // SSTORE
	}

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
			Name: "Withdawals Fork on Block 1",
			About: `
				Tests the withdrawals fork happening on block 1, Block 0 is for Aura.
				`,
		},
		WithdrawalsForkHeight: 1, //TODO
		WithdrawalsBlockCount: 1, // Genesis is not a withdrawals block
		WithdrawalsPerBlock:   16,
		TimeIncrements:        5,
	},

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdrawals Fork on Block 1",
	// 		About: `
	// 			Tests the withdrawals fork happening directly after genesis.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight: 1, // Only Genesis is Pre-Withdrawals
	// 	WithdrawalsBlockCount: 1,
	// 	WithdrawalsPerBlock:   16,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdrawals Fork on Block 5",
	// 		About: `
	// 			Tests the transition to the withdrawals fork after a single block
	// 			has happened.
	// 			Block 1 is sent with invalid non-null withdrawals payload and
	// 			client is expected to respond with the appropriate error.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight: 5, // Genesis and Block 1 are Pre-Withdrawals
	// 	WithdrawalsBlockCount: 1,
	// 	WithdrawalsPerBlock:   16,
	// 	TimeIncrements:        5,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdrawals Fork on Block 3",
	// 		About: `
	// 		Tests the transition to the withdrawals fork after two blocks
	// 		have happened.
	// 		Block 2 is sent with invalid non-null withdrawals payload and
	// 		client is expected to respond with the appropriate error.
	// 		`,
	// 	},
	// 	WithdrawalsForkHeight:    3, // Genesis, Block 1 and 2 are Pre-Withdrawals
	// 	WithdrawalsBlockCount:    1,
	// 	WithdrawalsPerBlock:      16,
	// 	TimeIncrements:           5,
	// 	TestCorrupedHashPayloads: true,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdraw to a single account",
	// 		About: `
	// 			Make multiple withdrawals to a single account.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight:    1,
	// 	WithdrawalsBlockCount:    1,
	// 	WithdrawalsPerBlock:      64,
	// 	WithdrawableAccountCount: 1,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdraw to two accounts",
	// 		About: `
	// 			Make multiple withdrawals to two different accounts, repeated in
	// 			round-robin.
	// 			Reasoning: There might be a difference in implementation when an
	// 			account appears multiple times in the withdrawals list but the list
	// 			is not in ordered sequence.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight:    1,
	// 	WithdrawalsBlockCount:    1,
	// 	WithdrawalsPerBlock:      64,
	// 	WithdrawableAccountCount: 2,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdraw many accounts",
	// 		About: `
	// 			Make multiple withdrawals to 1024 different accounts.
	// 			Execute many blocks this way.
	// 			`,
	// 		TimeoutSeconds: 3600,
	// 	},
	// 	WithdrawalsForkHeight:    1,
	// 	WithdrawalsBlockCount:    4,
	// 	WithdrawalsPerBlock:      1024,
	// 	WithdrawableAccountCount: 1024,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Withdraw zero amount",
	// 		About: `
	// 			Make multiple withdrawals where the amount withdrawn is 0.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight:    1,
	// 	WithdrawalsBlockCount:    1,
	// 	WithdrawalsPerBlock:      64,
	// 	WithdrawableAccountCount: 2,
	// 	WithdrawAmounts: []uint64{
	// 		0,
	// 		1,
	// 	},
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Empty Withdrawals",
	// 		About: `
	// 			Produce withdrawals block with zero withdrawals.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight: 1,
	// 	WithdrawalsBlockCount: 1,
	// 	WithdrawalsPerBlock:   0,
	// },

	// &WithdrawalsBaseSpec{
	// 	Spec: test.Spec{
	// 		Name: "Corrupted Block Hash Payload (INVALID)",
	// 		About: `
	// 			Send a valid payload with a corrupted hash using engine_newPayloadV2.
	// 			`,
	// 	},
	// 	WithdrawalsForkHeight:    1,
	// 	WithdrawalsBlockCount:    1,
	// 	TestCorrupedHashPayloads: true,
	// },

	//Block value tests
	//&BlockValueSpec{
	//	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	//		Spec: test.Spec{
	//			Name: "GetPayloadV2 Block Value",
	//			About: `
	//			Verify the block value returned in GetPayloadV2.
	//			`,
	//		},
	//		WithdrawalsForkHeight: 1,
	//		WithdrawalsBlockCount: 1,
	//	},
	//},

	// Sync Tests
	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account - No Transactions",
	// 			About: `
	// 			- Spawn a first client
	// 			- Go through withdrawals fork on Block 1
	// 			- Withdraw to a single account 16 times each block for 2 blocks
	// 			- Spawn a secondary client and send FCUV2(head)
	// 			- Wait for sync and verify withdrawn account's balance
	// 			`,
	// 			//TimeoutSeconds: 6000,
	// 		},
	// 		WithdrawalsForkHeight:    1,
	// 		WithdrawalsBlockCount:    2,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 1,
	// 	},
	// 	SyncSteps: 1,
	// },

	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 2 blocks - Withdrawals on Block 1 - Single Withdrawal Account",
	// 			About: `
	// 			- Spawn a first client
	// 			- Go through withdrawals fork on Block 1
	// 			- Withdraw to a single account 16 times each block for 2 blocks
	// 			- Spawn a secondary client and send FCUV2(head)
	// 			- Wait for sync and verify withdrawn account's balance
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight:    1,
	// 		WithdrawalsBlockCount:    2,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 1,
	// 	},
	// 	SyncSteps: 1,
	// },

	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 2 blocks - Withdrawals on Genesis - Single Withdrawal Account",
	// 			About: `
	// 			- Spawn a first client, with Withdrawals since genesis
	// 			- Withdraw to a single account 16 times each block for 2 blocks
	// 			- Spawn a secondary client and send FCUV2(head)
	// 			- Wait for sync and verify withdrawn account's balance
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight:    0,
	// 		WithdrawalsBlockCount:    2,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 1,
	// 	},
	// 	SyncSteps: 1,
	// },

	// // TODO:
	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts - No Transactions",
	// 			About: `
	// 		- Spawn a first client
	// 		- Go through withdrawals fork on Block 2
	// 		- Withdraw to 16 accounts each block for 2 blocks
	// 		- Spawn a secondary client and send FCUV2(head)
	// 		- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
	// 		`,
	// 		},
	// 		WithdrawalsForkHeight:    2,
	// 		WithdrawalsBlockCount:    2,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 16,
	// 		TransactionsPerBlock:     common.Big0,
	// 	},
	// 	SyncSteps: 1,
	// },

	// // TODO:
	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 2 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts",
	// 			About: `
	// 		- Spawn a first client
	// 		- Go through withdrawals fork on Block 2
	// 		- Withdraw to 16 accounts each block for 2 blocks
	// 		- Spawn a secondary client and send FCUV2(head)
	// 		- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
	// 		`,
	// 		},
	// 		WithdrawalsForkHeight:    2,
	// 		WithdrawalsBlockCount:    2,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 16,
	// 	},
	// 	SyncSteps: 1,
	// },

	// TODO: This test is failing, need to investigate.
	// &WithdrawalsSyncSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Sync after 128 blocks - Withdrawals on Block 2 - Multiple Withdrawal Accounts",
	// 			About: `
	// 		- Spawn a first client
	// 		- Go through withdrawals fork on Block 2
	// 		- Withdraw to many accounts 16 times each block for 128 blocks
	// 		- Spawn a secondary client and send FCUV2(head)
	// 		- Wait for sync, which include syncing a pre-Withdrawals block, and verify withdrawn account's balance
	// 		`,
	// 			TimeoutSeconds: 3600,
	// 		},
	// 		WithdrawalsForkHeight:    2,
	// 		WithdrawalsBlockCount:    128,
	// 		WithdrawalsPerBlock:      16,
	// 		WithdrawableAccountCount: 1024,
	// 	},
	// 	SyncSteps: 1,
	// },

	// // Re-Org tests
	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 1 Block Re-Org",
	// 			About: `
	// 			Tests a simple 1 block re-org
	// 			`,
	// 			SlotsToSafe:      big.NewInt(32),
	// 			SlotsToFinalized: big.NewInt(64),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 16,
	// 		WithdrawalsPerBlock:   16,
	// 	},
	// 	ReOrgBlockCount: 1,
	// 	ReOrgViaSync:    false,
	// },
	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org NewPayload",
	// 			About: `
	// 			Tests a 8 block re-org using NewPayload
	// 			Re-org does not change withdrawals fork height
	// 			`,
	// 			SlotsToSafe:      big.NewInt(32),
	// 			SlotsToFinalized: big.NewInt(64),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 16,
	// 		WithdrawalsPerBlock:   16,
	// 	},
	// 	ReOrgBlockCount: 8,
	// 	ReOrgViaSync:    false,
	// },
	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 8 Block Re-Org, Sync",
	// 			About: `
	// 			Tests a 8 block re-org using NewPayload
	// 			Re-org does not change withdrawals fork height
	// 			`,
	// 			SlotsToSafe:      big.NewInt(32),
	// 			SlotsToFinalized: big.NewInt(64),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 1, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 16,
	// 		WithdrawalsPerBlock:   16,
	// 	},
	// 	ReOrgBlockCount: 8,
	// 	ReOrgViaSync:    true,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org NewPayload",
	// 			About: `
	// 			Tests a 10 block re-org using NewPayload
	// 			Re-org does not change withdrawals fork height, but changes
	// 			the payload at the height of the fork
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 	},
	// 	ReOrgBlockCount: 10,
	// 	ReOrgViaSync:    false,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 8 - 10 Block Re-Org Sync",
	// 			About: `
	// 			Tests a 10 block re-org using sync
	// 			Re-org does not change withdrawals fork height, but changes
	// 			the payload at the height of the fork
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 	},
	// 	ReOrgBlockCount: 10,
	// 	ReOrgViaSync:    true,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org",
	// 			About: `
	// 			Tests a 10 block re-org using NewPayload
	// 			Sidechain reaches withdrawals fork at a lower block height
	// 			than the canonical chain
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 	},
	// 	ReOrgBlockCount:         10,
	// 	ReOrgViaSync:            false,
	// 	SidechainTimeIncrements: 2,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Canonical Block 8 / Side Block 7 - 10 Block Re-Org Sync",
	// 			About: `
	// 			Tests a 10 block re-org using sync
	// 			Sidechain reaches withdrawals fork at a lower block height
	// 			than the canonical chain
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 	},
	// 	ReOrgBlockCount:         10,
	// 	ReOrgViaSync:            true,
	// 	SidechainTimeIncrements: 2,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org",
	// 			About: `
	// 			Tests a 10 block re-org using NewPayload
	// 			Sidechain reaches withdrawals fork at a higher block height
	// 			than the canonical chain
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 		TimeIncrements:        2,
	// 	},
	// 	ReOrgBlockCount:         10,
	// 	ReOrgViaSync:            false,
	// 	SidechainTimeIncrements: 1,
	// },

	// &WithdrawalsReorgSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Canonical Block 8 / Side Block 9 - 10 Block Re-Org Sync",
	// 			About: `
	// 			Tests a 10 block re-org using sync
	// 			Sidechain reaches withdrawals fork at a higher block height
	// 			than the canonical chain
	// 			`,
	// 			SlotsToSafe:      big.NewInt(16),
	// 			SlotsToFinalized: big.NewInt(32),
	// 			TimeoutSeconds:   3600,
	// 		},
	// 		WithdrawalsForkHeight: 8, // Genesis is Pre-Withdrawals
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   128,
	// 		TimeIncrements:        2,
	// 	},
	// 	ReOrgBlockCount:         10,
	// 	ReOrgViaSync:            true,
	// 	SidechainTimeIncrements: 1,
	// },

	// // TODO: REORG SYNC WHERE SYNCED BLOCKS HAVE WITHDRAWALS BEFORE TIME

	// // EVM Tests (EIP-3651, EIP-3855, EIP-3860)
	// // &MaxInitcodeSizeSpec{
	// // 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// // 		Spec: test.Spec{
	// // 			Name: "Max Initcode Size",
	// // 		},
	// // 		WithdrawalsForkHeight: 2, // Block 1 is Pre-Withdrawals
	// // 		WithdrawalsBlockCount: 2,
	// // 	},
	// // 	OverflowMaxInitcodeTxCountBeforeFork: 0,
	// // 	OverflowMaxInitcodeTxCountAfterFork:  1,
	// // },

	// // Execution layer withdrawals spec
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 1 blocks withdrawals - 1 mass-claim",
	// 			About: `
	// 			- Shapella on block 1
	// 			- 1 block with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 1, // Genesis and Block 1 are Pre-Withdrawals
	// 		WithdrawalsBlockCount: 1,
	// 		WithdrawalsPerBlock:   16,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 1,
	// },
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 2 blocks withdrawals - 2 mass-claims",
	// 			About: `
	// 			- Shapella on block 1
	// 			- 2 blocks with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Produce 1 additional pair (A, B) of blocks:
	// 			A: block with withdrawals
	// 			B: block with claim Tx
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 1, // Genesis and Block 1 are Pre-Withdrawals
	// 		WithdrawalsBlockCount: 2,
	// 		WithdrawalsPerBlock:   16,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 2,
	// },
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 2 - 2 blocks withdrawals - 2 mass-claims",
	// 			About: `
	// 			- Shapella on block 2
	// 			- 2 blocks with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Produce 1 additional pair (A, B) of blocks:
	// 			A: block with withdrawals
	// 			B: block with claim Tx
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 2,
	// 		WithdrawalsBlockCount: 2,
	// 		WithdrawalsPerBlock:   16,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 2,
	// },
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 4 - 4 blocks withdrawals - 5 mass-claims - 64 withdrawals per block",
	// 			About: `
	// 			- Shapella on block 4
	// 			- 4 blocks with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Produce 4 additional pairs (A, B) of blocks:
	// 			  A: block with withdrawals
	// 			  B: block with claim Tx
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 4,
	// 		WithdrawalsBlockCount: 4,
	// 		WithdrawalsPerBlock:   64,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 5,
	// },

	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 5",
	// 			About: `
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 5,
	// 		WithdrawalsBlockCount: 1,
	// 		WithdrawalsPerBlock:   32,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 2,
	// },
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 2 - 8 blocks withdrawals - 2 mass-claims - 256 withdrawals per block",
	// 			About: `
	// 			- Shapella on block 2
	// 			- 8 blocks with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Produce 1 additional pair (A, B) of blocks:
	// 			A: block with withdrawals
	// 			B: block with claim Tx
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 2,
	// 		WithdrawalsBlockCount: 8,
	// 		WithdrawalsPerBlock:   256,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 2,
	// },
	// &WithdrawalsExecutionLayerSpec{
	// 	WithdrawalsBaseSpec: &WithdrawalsBaseSpec{
	// 		Spec: test.Spec{
	// 			Name: "Withdrawals Fork on Block 1 - 3 blocks withdrawals - 4 mass-claims - 1024 withdrawals per block",
	// 			About: `
	// 			- Shapella on block 1
	// 			- 3 blocks with withdrawals
	// 			- Claim accumulated withdrawals
	// 			- Produce 3 additional pairs (A, B) of blocks:
	// 			A: block with withdrawals
	// 			B: block with claim Tx
	// 			- Compares balances and events values with withdrawals from CL
	// 			`,
	// 		},
	// 		WithdrawalsForkHeight: 1,
	// 		WithdrawalsBlockCount: 3,
	// 		WithdrawalsPerBlock:   1024,
	// 		TimeIncrements:        5,
	// 	},
	// 	ClaimBlocksCount: 4,
	// },
}

// Helper types to convert gwei into wei more easily
func WeiAmount(w *types.Withdrawal) *big.Int {
	return new(big.Int).Mul(new(big.Int).SetUint64(w.Amount), big.NewInt(1e9))
}

// Helper structure used to keep history of the amounts withdrawn to each test account.
type WithdrawalsHistory map[uint64]types.Withdrawals

// Gets an accumulated account balance range of blocks 0 --> given block
func (wh WithdrawalsHistory) GetExpectedAccumulatedBalance(account common.Address, block uint64) *big.Int {
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

// Gets an accumulated account balance to a range fromBlock --> toBlock
func (wh WithdrawalsHistory) GetExpectedAccumulatedBalanceDelta(account common.Address, fromBlock, toBlock uint64) *big.Int {
	balance := big.NewInt(0)
	for b := fromBlock; b <= toBlock; b++ {
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
	for account := range accounts {
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

func (ws *WithdrawalsBaseSpec) GetPreShapellaBlockCount() int {
	return int(ws.WithdrawalsForkHeight)
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

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (ws *WithdrawalsBaseSpec) GetGenesisTest(base string) string {

	genesis := ws.Spec.GetGenesisTest(base)
	return genesis
}

// Append the accounts we are going to withdraw to, which should also include
// bytecode for testing purposes.
func (ws *WithdrawalsBaseSpec) GetGenesis(base string) helper.Genesis {

	genesis := ws.Spec.GetGenesis(base)

	warmCoinbaseAcc := helper.NewAccount()
	push0Acc := helper.NewAccount()

	warmCoinbaseAcc.SetBalance(common.Big0)
	warmCoinbaseAcc.SetCode(warmCoinbaseCode)

	genesis.AllocGenesis(WARM_COINBASE_ADDRESS, warmCoinbaseAcc)

	push0Acc.SetBalance(common.Big0)
	push0Acc.SetCode(push0Code)

	genesis.AllocGenesis(PUSH0_ADDRESS, push0Acc)
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
	}
	return ws.WithdrawalsForkHeight - 1

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
			2,
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

// sendPayloadTransactions spreads and sends TransactionCountPerPayload equaly between TX_CONTRACT_ADDRESSES
//
// Tx params:
//
//	Amount:    common.Big1
//	Payload:   nil
//	TxType:    t.TestTransactionType
//	GasLimit:  t.Genesis.GasLimit()
//	ChainID:   t.Genesis.Config().ChainID,
func (ws *WithdrawalsBaseSpec) sendPayloadTransactions(t *test.Env) {
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
				// TODO: figure out why contract storage check fails on block 2 with Genesis.GasLimit()
				GasLimit: 75000,
				ChainID:  t.Genesis.Config().ChainID,
			},
		)

		if err != nil {
			t.Fatalf("FAIL (%s): Error trying to send transaction: %v", t.TestName, err)
		}
	}
}

// waitForSetup sleeps for some time to allow clients start properly and prepare for execution.
// This function also sets timestamp for genesis block == (end of setup wait):
//
// Shapella timestamp == SetupTime + (pre-shapella blocks count * blockTimeIncrements)
// => genesis block timestamp == shapella timestamp - (pre-shapella blocks count * blockTimeIncrements)
func (ws *WithdrawalsBaseSpec) waitForSetup(t *test.Env) {
	preShapellaBlocksTime := time.Duration(uint64(ws.GetPreShapellaBlockCount())*ws.GetBlockTimeIncrements()) * time.Second
	endOfSetupTimestamp := time.Unix(int64(*t.Genesis.Config().ShanghaiTime), 0).Add(-preShapellaBlocksTime)
	defer func() {
		t.CLMock.LatestHeader.Time = uint64(endOfSetupTimestamp.Unix())
	}()
	if time.Now().Unix() < endOfSetupTimestamp.Unix() {
		durationUntilFuture := time.Until(endOfSetupTimestamp)
		if durationUntilFuture > 0 {
			t.Logf("INFO: Waiting for setup: ~ %.2f min...", durationUntilFuture.Minutes())
			time.Sleep(durationUntilFuture)
		}
	}
}

// Base test case execution procedure for withdrawals
func (ws *WithdrawalsBaseSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)
	shangaiTime := *t.Genesis.Config().ShanghaiTime
	t.CLMock.ShanghaiTimestamp = big.NewInt(0).SetUint64(shangaiTime)

	// Wait ttd
	t.CLMock.WaitForTTD()
	ws.waitForSetup(t)
	r := t.TestEngine.TestBlockByNumber(nil)
	r.ExpectationDescription = `
	Requested "latest" block expecting genesis to contain
	withdrawalRoot=nil, because genesis.timestamp < shanghaiTime
	`
	r.ExpectWithdrawalsRoot(nil)

	// Produce any blocks necessary to reach withdrawals fork
	t.CLMock.ProduceBlocks(int(ws.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {

			ws.sendPayloadTransactions(t)

			if !ws.SkipBaseVerifications {

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
				r.ExpectationDescription = "Sent pre-shanghai Forkchoice ForkchoiceUpdatedV2 + null withdrawals, no error is expected"
				r.ExpectNoError()
			}
		},
		OnGetPayload: func() {
			if !ws.SkipBaseVerifications {
				// Try to get the same payload but use `engine_getPayloadV2`
				g := t.TestEngine.TestEngineGetPayloadV2(t.CLMock.NextPayloadID)
				g.ExpectPayload(&t.CLMock.LatestPayloadBuilt)
				g.ExpectNoError()

				// Send valid ExecutionPayloadV1 using engine_newPayloadV2
				r := t.TestEngine.TestEngineNewPayloadV2(&t.CLMock.LatestPayloadBuilt)
				r.ExpectationDescription = "Sent pre-shanghai payload using NewPayloadV2, no error is expected"
				r.ExpectNoError()
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

	var (
		startAccount = ws.GetWithdrawalsStartAccount()
		nextIndex    = uint64(0)
	)

	// start client
	client := getClient(t)
	if client == nil {
		t.Fatalf("Couldn't connect to client")
		return
	}
	defer client.Close()

	// Produce requested post-shanghai blocks:
	// 1. Send some withdrawals and transactions
	// 2. Check that contract withdrawable amount matches the expectations
	for i := 0; i < int(ws.WithdrawalsBlockCount); i++ {
		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// Send some withdrawals
				t.CLMock.NextWithdrawals, nextIndex = ws.GenerateWithdrawalsForBlock(nextIndex, startAccount)
				ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals

				ws.sendPayloadTransactions(t)
			},
			OnGetPayload: func() {
				if !ws.SkipBaseVerifications {

					// Verify the list of withdrawals returned on the payload built
					// completely matches the list provided in the
					// engine_forkchoiceUpdatedV2 method call
					if sentList, ok := ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber]; !ok {
						t.Fatalf("FAIL (%s): Withdrawals sent list was not saved", t.TestName)
					} else {
						if len(sentList) != len(t.CLMock.LatestPayloadBuilt.Withdrawals) {
							t.Fatalf(
								"FAIL (%s): Incorrect list of withdrawals on built payload: want=%d, got=%d",
								t.TestName,
								len(sentList),
								len(t.CLMock.LatestPayloadBuilt.Withdrawals),
							)
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
					if ws.TestCorrupedHashPayloads {
						payload := t.CLMock.LatestExecutedPayload

						// Corrupt the hash
						_, _ = rand.Read(payload.BlockHash[:])

						// On engine_newPayloadV2 `INVALID_BLOCK_HASH` is deprecated
						// in favor of reusing `INVALID`
						n := t.TestEngine.TestEngineNewPayloadV2(&payload)
						n.ExpectStatus(test.Invalid)
					}
				}
			},
			OnForkchoiceBroadcast: func() {
				if !ws.SkipBaseVerifications {
					for _, addr := range ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(t.CLMock.LatestExecutedPayload.Number) {
						// Test balance at `latest`, which should have the withdrawal applied.
						withdrawableAmount, err := getWithdrawableAmount(client, addr, big.NewInt(int64(t.CLMock.LatestExecutedPayload.Number)))
						if err != nil {
							t.Fatalf("FAIL (%s): Error trying to get balance of token: %v, address: %v", t.TestName, err, addr.Hex())
						}

						expectBalanceMGNO := ws.WithdrawalsHistory.GetExpectedAccumulatedBalance(addr, t.CLMock.LatestExecutedPayload.Number)
						expectBalance := libgno.UnwrapToGNO(expectBalanceMGNO)

						if withdrawableAmount.Cmp(expectBalance) != 0 {
							t.Fatalf(
								"FAIL (%s): Incorrect balance on account %s after withdrawals applied: want=%d, got=%d",
								t.TestName,
								addr,
								expectBalance,
								withdrawableAmount,
							)
						}
					}
					ws.VerifyContractsStorage(t)
				}
			},
		})
	}

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

func getClient(t *test.Env) *ethclient.Client {
	url, _ := t.CLMock.EngineClients[0].Url()
	client, err := ethclient.Dial(url)
	if err != nil {
		return nil
	}
	return client
}

// getBalanceChangeDelta calls balanceOf for fromBlock and toBlock heights and returns balance delta
// i.e. how much balance changed from block X to block Y
func getBalanceChangeDelta(client *ethclient.Client, account common.Address, fromBlock, toBlock *big.Int) (*big.Int, error) {
	fromBalance, err := libgno.GetBalanceOf(client, account, fromBlock)
	if err != nil {
		return nil, err
	}
	toBalance, err := libgno.GetBalanceOf(client, account, toBlock)
	if err != nil {
		return nil, err
	}
	diff := big.NewInt(0).Sub(toBalance, fromBalance)
	// if diff is positive return it, othervise return -(diff)
	if toBalance.Cmp(fromBalance) == 1 {
		return diff, nil
	}
	return diff.Neg(diff), nil
}

// getWithdrawableAmount returns withdrawableAmount for specific address from deposit contract
func getWithdrawableAmount(client *ethclient.Client, account common.Address, block *big.Int) (*big.Int, error) {
	withdrawalsABI, err := libgno.GetWithdrawalsABI()
	if err != nil {
		return nil, err
	}
	var result []interface{}
	withdrawalsContract := bind.NewBoundContract(libgno.WithdrawalsContractAddress, *withdrawalsABI, client, client, client)
	opts := &bind.CallOpts{Pending: false, BlockNumber: block}
	err = withdrawalsContract.Call(opts, &result, "withdrawableAmount", account)
	if err != nil {
		return nil, err
	}
	if len(result) != 1 {
		return nil, fmt.Errorf("unexpected result length: %d", len(result))
	}
	return result[0].(*big.Int), nil
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
	//var secondaryEngineTestChan chan *test.TestEngineClient
	//var secondaryEngineChan chan client.EngineClient
	//ws.WithdrawalsBaseSpec.SkipBaseVerifications = true
	//ws.WithdrawalsBaseSpec.Execute(t)
	//go func() {
	//	if err != nil {
	//		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	//	}
	//	secondaryEngineChan <- secondaryEngine
	//	secondaryEngineTestChan <- test.NewTestEngineClient(t, secondaryEngine)
	//
	//}()
	//// Do the base withdrawal test first, skipping base verifications
	//secondaryEngine := <-secondaryEngineChan
	//secondaryEngineTest := <-secondaryEngineTestChan
	ws.WithdrawalsBaseSpec.SkipBaseVerifications = true
	ws.WithdrawalsBaseSpec.Execute(t)

	// Spawn a secondary client which will need to sync to the primary client
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{TerminalTotalDifficulty: t.Genesis.Difficulty()}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
	//secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	t.CLMock.AddEngineClient(secondaryEngine)

	ctx := context.Background()
	block1, err := t.CLMock.EngineClients[0].BlockByNumber(ctx, nil)
	if err != nil {
		panic("failed to get block by number")
	}

	t.CLMock.LatestForkchoice.HeadBlockHash = block1.Hash()

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
				r.ExpectNoError()
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
	ctx = context.Background()
	block1, err = t.CLMock.EngineClients[0].BlockByNumber(ctx, nil)
	if err != nil {
		panic("failed to get block by number")
	}
	block2, err := secondaryEngine.BlockByNumber(ctx, nil)
	if err != nil {
		panic("failed to get block by number")
	}
	// Latest block in both EngineClients should be the same
	if block1.Number().Uint64() != block2.Number().Uint64() {
		t.Fatalf("FAIL (%s): Secondary client is not synced to the same block as the primary client", t.TestName)
	}
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
	shangaiTime := *t.Genesis.Config().ShanghaiTime
	t.CLMock.ShanghaiTimestamp = big.NewInt(0).SetUint64(shangaiTime)

	t.CLMock.WaitForTTD()

	// Spawn a secondary client which will produce the sidechain
	secondaryEngine, err := hive_rpc.HiveRPCEngineStarter{}.StartClient(t.T, t.TestContext, t.Genesis, t.ClientParams, t.ClientFiles, t.Engine)
	if err != nil {
		t.Fatalf("FAIL (%s): Unable to spawn a secondary client: %v", t.TestName, err)
	}
	secondaryEngineTest := test.NewTestEngineClient(t, secondaryEngine)
	// t.CLMock.AddEngineClient(secondaryEngine)

	var (
		canonicalStartAccount       = ws.GetWithdrawalsStartAccount()
		canonicalNextIndex          = uint64(0)
		sidechainStartAccount       = new(big.Int).SetBit(common.Big0, 160, 1)
		sidechainNextIndex          = uint64(0)
		sidechainWithdrawalsHistory = make(WithdrawalsHistory)
		sidechain                   = make(map[uint64]*beacon.ExecutableData)
		sidechainPayloadId          *beacon.PayloadID
	)

	// Sidechain withdraws on the max account value range 0xffffffffffffffffffffffffffffffffffffffff
	sidechainStartAccount.Sub(sidechainStartAccount, big.NewInt(int64(ws.GetWithdrawableAccountCount())+1))
	ws.waitForSetup(t)
	for i := 0; i < int(ws.GetPreWithdrawalsBlockCount()+ws.WithdrawalsBlockCount); i++ {

		t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
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
				isShanghai := shangaiTime <= uint64(time.Now().Unix())
				_ = isShanghai
				withdrawalsForkHeight := ws.GetSidechainWithdrawalsForkHeight()
				_ = withdrawalsForkHeight
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
	}

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
			t.Logf(
				"INFO (%s): Sending sidechain payload %d, hash=%s, parent=%s",
				t.TestName,
				payloadNumber,
				payload.BlockHash,
				payload.ParentHash,
			)
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

// Withdrawals execution layer spec:
type WithdrawalsExecutionLayerSpec struct {
	*WithdrawalsBaseSpec
	ClaimBlocksCount int
}

func (ws *WithdrawalsExecutionLayerSpec) claimBlocksCount() int {
	if ws.ClaimBlocksCount == 0 {
		return 1
	}
	return ws.ClaimBlocksCount
}

// ClaimWithdrawals sends claimWithdrawals() call to deposit contract with list of addresses
// from withdrawals history
func (ws *WithdrawalsBaseSpec) ClaimWithdrawals(t *test.Env) {
	// Get ExecuteWithdrawalsClaims
	addresses := make([]common.Address, 0)
	for _, w := range ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber-1] {
		addresses = append(addresses, w.Address)
	}
	// Send claim transaction
	claims, err := libgno.ClaimWithdrawalsData(addresses)
	if err != nil {
		t.Fatalf("FAIL (%s): Cant create claimWithdrawals transaction payload: %v", t.TestName, err)
	}
	_, err = helper.SendNextTransactionWithAccount(
		t.TestContext,
		t.CLMock.NextBlockProducer,
		&helper.BaseTransactionCreator{
			Recipient:  &libgno.WithdrawalsContractAddress,
			Amount:     common.Big0,
			Payload:    claims,
			PrivateKey: globals.GnoVaultVaultKey,
			TxType:     t.TestTransactionType,
			GasLimit:   t.Genesis.GasLimit(),
			ChainID:    t.Genesis.Config().ChainID,
		},
		globals.GnoVaultAccountAddress,
	)
	if err != nil {
		t.Fatalf("FAIL (%s): Error trying to send claim transaction: %v", t.TestName, err)
	}
}

// VerifyClaimsExecution verifies that:
//
// sum(withdrawals) == ERC20 balance delta == sum(transfer events values)
//
// for provided block range
func (ws *WithdrawalsExecutionLayerSpec) VerifyClaimsExecution(
	t *test.Env, client *ethclient.Client, fromBlock, toBlock uint64,
) {
	addresses := ws.WithdrawalsHistory.GetAddressesWithdrawnOnBlock(toBlock - 1)
	transfersMap, err := libgno.GetWithdrawalsTransferEvents(client, addresses, fromBlock, toBlock)
	if err != nil {
		t.Fatalf("FAIL (%s): Error trying to get claims transfer events: %w", t.TestName, err)
	}
	if len(addresses) == 0 {
		t.Fatalf("FAIL (%s): No withdrawal addresses found: %w", t.TestName)
	}
	for _, addr := range addresses {
		balanceDelta, err := getBalanceChangeDelta(client, addr, big.NewInt(int64(fromBlock)), big.NewInt(int64(toBlock)))
		if err != nil {
			t.Fatalf("FAIL (%s): Error trying to get balance delta of token: %v, address: %v, from block %d to block %d", t.TestName, err, addr.Hex(), fromBlock, toBlock)
		}

		withdrawalsAccumulatedDeltaMGNO := ws.WithdrawalsHistory.GetExpectedAccumulatedBalanceDelta(addr, fromBlock, toBlock)
		withdrawalsDeltaGNO := libgno.UnwrapToGNO(withdrawalsAccumulatedDeltaMGNO)

		// check that account balance == expected balance from withdrawals history
		if balanceDelta.Cmp(withdrawalsDeltaGNO) != 0 {
			t.Fatalf(
				"FAIL (%s): Incorrect balance on account %s after withdrawals applied: want=%d, got=%d",
				t.TestName, addr, balanceDelta, withdrawalsDeltaGNO,
			)
		}
		// if block range is >1 there will be the trasfered sum from all transfer events
		// for specific address in block range
		eventValue := transfersMap[addr.Hex()]
		if eventValue == nil {
			t.Fatalf(
				"FAIL (%s): No withdrawal transfer value presented in events list for address: %v",
				t.TestName, addr.Hex(),
			)
		}
		// check that account balance == value from transfer event
		if balanceDelta.Cmp(eventValue) != 0 {
			t.Fatalf(
				"FAIL (%s): Transfer event value is not equal latest balance for address %s: want=%d (actual), got=%d (from event)",
				t.TestName, addr.Hex(), balanceDelta, transfersMap[addr.Hex()],
			)
		}
	}
}

func (ws *WithdrawalsExecutionLayerSpec) Execute(t *test.Env) {
	// Create the withdrawals history object
	ws.WithdrawalsHistory = make(WithdrawalsHistory)
	shangaiTime := *t.Genesis.Config().ShanghaiTime
	t.CLMock.ShanghaiTimestamp = big.NewInt(0).SetUint64(shangaiTime)

	// Wait ttd
	t.CLMock.WaitForTTD()

	r := t.TestEngine.TestBlockByNumber(nil)
	r.ExpectationDescription = `
	Requested "latest" block expecting genesis to contain
	withdrawalRoot=nil, because genesis.timestamp < shanghaiTime
	`
	r.ExpectWithdrawalsRoot(nil)
	ws.waitForSetup(t)
	// Produce any blocks necessary to reach withdrawals fork
	t.CLMock.ProduceBlocks(int(ws.GetPreWithdrawalsBlockCount()), clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			if !ws.SkipBaseVerifications {
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
				r.ExpectationDescription = "Sent pre-shanghai Forkchoice ForkchoiceUpdatedV2 + null withdrawals, no error is expected"
				r.ExpectNoError()
			}
		},
	})

	var (
		startAccount = ws.GetWithdrawalsStartAccount()
		nextIndex    = uint64(0)
	)

	// start client
	client := getClient(t)
	if client == nil {
		t.Fatalf("Couldn't connect to client")
		return
	}
	defer client.Close()

	// Produce requested post-shanghai blocks:
	// 1. Produce WithdrawalsBlockCount number of blocks with withdrawals
	// 2. Claim accumulated withdrawals and verify claims execution and balances
	// 3. Iteratively produce block pairs (first with withdrawals and second with claim Tx)
	//    and verify every claim execution
	for i := 0; i < int(ws.WithdrawalsBlockCount); i++ {
		t.CLMock.ProduceBlocks(int(ws.WithdrawalsBlockCount), clmock.BlockProcessCallbacks{
			OnPayloadProducerSelected: func() {
				// Send some withdrawals
				t.CLMock.NextWithdrawals, nextIndex = ws.GenerateWithdrawalsForBlock(nextIndex, startAccount)
				ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
			},
		})
	}

	t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
		OnPayloadProducerSelected: func() {
			ws.ClaimWithdrawals(t)
		},
		OnForkchoiceBroadcast: func() {
			ws.VerifyClaimsExecution(t, client, ws.WithdrawalsForkHeight, t.CLMock.LatestExecutedPayload.Number)
		},
	})

	// performs multiple iterations of withdrawals generation and claiming
	if !ws.SkipBaseVerifications && ws.claimBlocksCount() > 1 {
		for i := 1; i <= ws.claimBlocksCount(); i++ {
			t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
				OnPayloadProducerSelected: func() {
					// Send some withdrawals
					t.CLMock.NextWithdrawals, nextIndex = ws.GenerateWithdrawalsForBlock(nextIndex, startAccount)
					ws.WithdrawalsHistory[t.CLMock.CurrentPayloadNumber] = t.CLMock.NextWithdrawals
				},
			})
			t.CLMock.ProduceSingleBlock(clmock.BlockProcessCallbacks{
				OnPayloadProducerSelected: func() {
					ws.ClaimWithdrawals(t)
				},
				OnForkchoiceBroadcast: func() {
					ws.VerifyClaimsExecution(t, client, t.CLMock.LatestExecutedPayload.Number-1, t.CLMock.LatestExecutedPayload.Number)
				},
			})

		}
	}
	if !ws.SkipBaseVerifications {
		ws.VerifyClaimsExecution(t, client, ws.WithdrawalsForkHeight, t.CLMock.LatestExecutedPayload.Number)
	}
}
