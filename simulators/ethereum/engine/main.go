package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	// Test chain parameters
	chainID   = big.NewInt(7)
	gasPrice  = big.NewInt(30 * params.GWei)
	networkID = big.NewInt(7)

	// PoS related
	PoSBlockProductionPeriod = time.Second * 1
	tTDCheckPeriod           = time.Second * 1
	DefaultTestCaseTimeout   = time.Second * 60
	DefaultPoSSyncTimeout    = time.Second * 20

	// Confirmation blocks
	PoWConfirmationBlocks = uint64(15)
	PoSConfirmationBlocks = uint64(1)
)

var vault *Vault

var clientEnv = hivesim.Params{
	"HIVE_NETWORK_ID":          networkID.String(),
	"HIVE_CHAIN_ID":            chainID.String(),
	"HIVE_FORK_HOMESTEAD":      "0",
	"HIVE_FORK_TANGERINE":      "0",
	"HIVE_FORK_SPURIOUS":       "0",
	"HIVE_FORK_BYZANTIUM":      "0",
	"HIVE_FORK_CONSTANTINOPLE": "0",
	"HIVE_FORK_PETERSBURG":     "0",
	"HIVE_FORK_ISTANBUL":       "0",
	"HIVE_FORK_MUIR_GLACIER":   "0",
	"HIVE_FORK_BERLIN":         "0",
	"HIVE_FORK_LONDON":         "0",
	// Tests use clique PoA to mine new blocks until the TTD is reached, then PoS takes over.
	"HIVE_CLIQUE_PERIOD":     "1",
	"HIVE_CLIQUE_PRIVATEKEY": "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c",
	"HIVE_MINER":             "658bdf435d810c91414ec09147daa6db62406379",
	// Merge related
	"HIVE_MERGE_BLOCK_ID": "100",
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

type TestSpec struct {
	Name  string
	About string
	Run   func(*TestEnv)
	TTD   int64
}

var tests = []TestSpec{

	// Engine API Negative Test Cases
	{
		Name: "Engine API Proof of Work",
		Run:  engineAPIPoWTests,
		TTD:  1000000,
	},
	{
		Name: "Unknown HeadBlockHash",
		Run:  unknownHeadBlockHash,
	},
	{
		Name: "Unknown SafeBlockHash",
		Run:  unknownSafeBlockHash,
	},
	{
		Name: "Unknown FinalizedBlockHash",
		Run:  unknownFinalizedBlockHash,
	},
	{
		Name: "Pre-TTD ForkchoiceUpdated After PoS Switch",
		Run:  preTTDFinalizedBlockHash,
		TTD:  2,
	},
	{
		Name: "Bad Hash on ExecutePayload",
		Run:  badHashOnExecPayload,
	},
	{
		Name: "Invalid ParentHash ExecutePayload",
		Run:  invalidPayloadTestCaseGen("ParentHash"),
	},
	{
		Name: "Invalid StateRoot ExecutePayload",
		Run:  invalidPayloadTestCaseGen("StateRoot"),
	},
	{
		Name: "Invalid ReceiptsRoot ExecutePayload",
		Run:  invalidPayloadTestCaseGen("ReceiptsRoot"),
	},
	{
		Name: "Invalid Number ExecutePayload",
		Run:  invalidPayloadTestCaseGen("Number"),
	},
	{
		Name: "Invalid GasLimit ExecutePayload",
		Run:  invalidPayloadTestCaseGen("GasLimit"),
	},
	{
		Name: "Invalid GasUsed ExecutePayload",
		Run:  invalidPayloadTestCaseGen("GasUsed"),
	},
	{
		Name: "Invalid Timestamp ExecutePayload",
		Run:  invalidPayloadTestCaseGen("Timestamp"),
	},

	// Eth RPC Status on ForkchoiceUpdated Events
	{
		Name: "Latest Block after ExecutePayload",
		Run:  blockStatusExecPayload,
	},
	{
		Name: "Latest Block after New HeadBlock",
		Run:  blockStatusHeadBlock,
	},
	{
		Name: "Latest Block after New SafeBlock",
		Run:  blockStatusSafeBlock,
	},
	{
		Name: "Latest Block after New FinalizedBlock",
		Run:  blockStatusFinalizedBlock,
	},
	{
		Name: "Latest Block after Reorg",
		Run:  blockStatusReorg,
	},

	// Payload Tests
	{
		Name: "Re-Execute Payload",
		Run:  reExecPayloads,
	},
	{
		Name: "Multiple New Payloads Extending Canonical Chain",
		Run:  multipleNewCanonicalPayloads,
	},

	// Transaction Reorg using Engine API
	{
		Name: "Transaction reorg",
		Run:  transactionReorg,
	},

	// Suggested Fee Recipient in Payload creation
	{
		Name: "Suggested Fee Recipient Test",
		Run:  suggestedFeeRecipient,
	},

	// Random opcode tests
	{
		Name: "Random Opcode Transactions",
		Run:  randomOpcodeTx,
	},

	// Multi-Client Sync tests
	// TODO ...
}

func main() {
	suite := hivesim.Suite{
		Name: "engine",
		Description: `
Test Engine API tests using CL mocker to inject commands into clients after they 
have reached the Terminal Total Difficulty.`[1:],
	}
	for _, currentTest := range tests {
		currentTest := currentTest
		newParams := clientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", currentTest.TTD))
		suite.Add(hivesim.ClientTestSpec{
			Name:        currentTest.Name,
			Description: currentTest.About,
			Parameters:  newParams,
			Files:       files,
			Run: func(t *hivesim.T, c *hivesim.Client) {
				// Run the test case
				RunTest(currentTest.Name, big.NewInt(currentTest.TTD), t, c, currentTest.Run)
			},
		})
	}
	hivesim.MustRunSuite(hivesim.New(), suite)
}
