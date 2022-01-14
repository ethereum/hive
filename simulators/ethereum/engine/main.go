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
	terminalTotalDifficulty  = big.NewInt(131072 + 25)
	PoSBlockProductionPeriod = time.Second * 1
	tTDCheckPeriod           = time.Second * 1

	// Confirmation blocks
	PoWConfirmationBlocks = uint64(15)
	PoSConfirmationBlocks = uint64(1)
)

var clMocker *CLMocker
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
	"HIVE_TERMINAL_TOTAL_DIFFICULTY": terminalTotalDifficulty.String(),
	"HIVE_MERGE_BLOCK_ID":            "100",
}

var files = map[string]string{
	"/genesis.json": "./init/genesis.json",
}

type testSpec struct {
	Name  string
	About string
	Run   func(*TestEnv)
}

var tests = []testSpec{

	// Engine API Negative Test Cases
	{Name: "Engine API Proof of Work", Run: engineAPIPoWTests},
	{Name: "Unknown HeadBlockHash", Run: unknownHeadBlockHash},
	{Name: "Unknown SafeBlockHash", Run: unknownSafeBlockHash},
	{Name: "Unknown FinalizedBlockHash", Run: unknownFinalizedBlockHash},
	{Name: "Pre-TTD ForkchoiceUpdated After PoS Switch", Run: preTTDFinalizedBlockHash},
	{Name: "Bad Hash on ExecutePayload", Run: badHashOnExecPayload},

	// Eth RPC Status on ForkchoiceUpdated Events
	{Name: "Latest Block after ExecutePayload", Run: blockStatusExecPayload},
	{Name: "Latest Block after New HeadBlock", Run: blockStatusHeadBlock},
	{Name: "Latest Block after New SafeBlock", Run: blockStatusSafeBlock},
	{Name: "Latest Block after New FinalizedBlock", Run: blockStatusFinalizedBlock},
	{Name: "Latest Block after Reorg", Run: blockStatusReorg},

	// Re-ExecutePayload
	{Name: "Re-Execute Payload", Run: reExecPayloads},

	// Transaction Reorg using Engine API
	{Name: "Transaction reorg", Run: transactionReorg},

	// Suggested Fee Recipient in Payload creation
	{Name: "Suggested Fee Recipient Test", Run: suggestedFeeRecipient},

	// Random opcode tests
	{Name: "Random Opcode Transactions", Run: randomOpcodeTx},
}

func main() {
	suite := hivesim.Suite{
		Name: "engine",
		Description: `
Test Engine API tests using CL mocker to inject commands into clients after they 
have reached the Terminal Total Difficulty.`[1:],
	}
	suite.Add(&hivesim.ClientTestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Parameters:  clientEnv,
		Files:       files,
		Run:         runSourceTest,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

// First launch a client against other clients will sync and run their respective tests.
func runSourceTest(t0 *hivesim.T, c0 *hivesim.Client) {
	clMocker = NewCLMocker()
	defer clMocker.stopPoSBlockProduction()

	vault = newVault()

	ec := NewEngineClient(t0, c0)
	clMocker.AddEngineClient(ec)

	enode, err := c0.EnodeURL()
	if err != nil {
		t0.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	newParams := clientEnv.Set("HIVE_BOOTNODE", enode)

	availableClients, err := t0.Sim.ClientTypes()
	if err != nil {
		t0.Fatal("could not get available clients:", err)
	}
	for _, clientType := range availableClients {
		for _, currentTest := range tests {
			t0.RunClient(clientType, hivesim.ClientTestSpec{
				Name:        fmt.Sprintf("%s", c0.Type),
				Description: "test",
				Parameters:  newParams,
				Files:       files,
				Run: func(t *hivesim.T, c *hivesim.Client) {
					// Create an Engine API wrapper for this client
					ec := NewEngineClient(t, c)
					defer ec.Close()
					// Add this client to CLMocker
					clMocker.AddEngineClient(ec)
					defer clMocker.RemoveEngineClient(ec)
					// Run the test case
					t.Run(hivesim.TestSpec{
						Name:        fmt.Sprintf("%s (%s)", currentTest.Name, c.Type),
						Description: currentTest.About,
						Run: func(t *hivesim.T) {
							runTest(currentTest.Name, t, c, vault, clMocker, currentTest.Run)
						},
					})
				},
			})
		}
	}

}
