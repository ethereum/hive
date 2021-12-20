package main

import (
	"context"
	"fmt"
	"math/big"
	"sync"
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

	// Eth RPC Status on ForkchoiceUpdated Events
	{Name: "Latest Block after ExecutePayload", Run: blockStatusExecPayload},
	{Name: "Latest Block after New HeadBlock", Run: blockStatusHeadBlock},
	{Name: "Latest Block after New SafeBlock", Run: blockStatusSafeBlock},
	{Name: "Latest Block after New FinalizedBlock", Run: blockStatusFinalizedBlock},
	{Name: "Latest Block after Reorg", Run: blockStatusReorg},

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
func runSourceTest(t *hivesim.T, c *hivesim.Client) {
	clMocker = NewCLMocker()
	vault = newVault()

	ec := NewEngineClient(t, c)
	clMocker.AddEngineClient(ec)

	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal("can't get node peer-to-peer endpoint:", enode)
	}
	newParams := clientEnv.Set("HIVE_BOOTNODE", enode)

	t.RunAllClients(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("%s", c.Type),
		Description: "test",
		Parameters:  newParams,
		Files:       files,
		Run:         runAllTests,
	})
	clMocker.stopPoSBlockProduction()
}

// Run all tests against a second running client which must sync at some point to the PoS chain.
func runAllTests(t *hivesim.T, c *hivesim.Client) {
	// Create an Engine API wrapper for this client
	ec := NewEngineClient(t, c)
	defer ec.Close()

	// Add this client to CLMocker
	clMocker.AddEngineClient(ec)
	defer clMocker.RemoveEngineClient(ec)

	// Setup wait lock for this client reaching PoS sync
	var cancelFP *context.CancelFunc
	var m sync.Mutex
	var st bool
	go func() {
		WaitClientForPoSSync(t, c, &cancelFP, &m, &st)
	}()

	// Launch all tests
	var wg sync.WaitGroup
	for _, test := range tests {
		wg.Add(1)
		test := test
		go func() {
			defer wg.Done()
			t.Run(hivesim.TestSpec{
				Name:        fmt.Sprintf("%s (%s)", test.Name, c.Type),
				Description: test.About,
				Run: func(t *hivesim.T) {
					runTest(test.Name, t, c, ec, vault, clMocker, &m, &st, test.Run)
				},
			})
		}()
	}
	if cancelFP != nil {
		(*cancelFP)()
	}

	// Wait for all test cases to complete
	wg.Wait()
}
