package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

var (
	// Test chain parameters
	chainID   = big.NewInt(7)
	gasPrice  = big.NewInt(30 * params.GWei)
	networkID = big.NewInt(7)

	// Time between checks of a client reaching Terminal Total Difficulty
	tTDCheckPeriod = time.Second * 1

	// Global test case timeout
	DefaultTestCaseTimeout = time.Second * 60

	// Time delay between ForkchoiceUpdated and GetPayload to allow the clients
	// to produce a new Payload
	PayloadProductionClientDelay = time.Second

	// Confirmation blocks
	PoWConfirmationBlocks = uint64(15)
	PoSConfirmationBlocks = uint64(1)

	// Test related
	prevRandaoContractAddr = common.HexToAddress("0000000000000000000000000000000000000316")

	// Clique Related
	minerPKHex   = "9c647b8b7c4e7c3490668fb6c11473619db80c93704c70893d3813af4090c39c"
	minerAddrHex = "658bdf435d810c91414ec09147daa6db62406379"

	// JWT Authentication Related
	defaultJwtTokenSecretBytes = []byte("secretsecretsecretsecretsecretse") // secretsecretsecretsecretsecretse
	maxTimeDriftSeconds        = int64(5)
)

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
	"HIVE_CLIQUE_PRIVATEKEY": minerPKHex,
	"HIVE_MINER":             minerAddrHex,
	// Merge related
	"HIVE_MERGE_BLOCK_ID": "100",
}

type TestSpec struct {
	// Name of the test
	Name string

	// Brief description of the test
	About string

	// Test procedure to execute the test
	Run func(*TestEnv)

	// TerminalTotalDifficulty delta value.
	// Actual TTD is genesis.Difficulty + this value
	// Default: 0
	TTD int64

	// CL Mocker configuration for slots to `safe` and `finalized` respectively
	SlotsToSafe      *big.Int
	SlotsToFinalized *big.Int

	// Test maximum execution time until a timeout is raised.
	// Default: 60 seconds
	TimeoutSeconds int

	// Genesis file to be used for all clients launched during test
	// Default: genesis.json (init/genesis.json)
	GenesisFile string

	// Chain file to initialize the clients.
	// When used, clique consensus mechanism is disabled.
	// Default: None
	ChainFile string
}

func main() {
	var (
		engine = hivesim.Suite{
			Name: "engine",
			Description: `
	Test Engine API tests using CL mocker to inject commands into clients after they 
	have reached the Terminal Total Difficulty.`[1:],
		}
		transition = hivesim.Suite{
			Name: "transition",
			Description: `
	Test Engine API tests using CL mocker to inject commands into clients and drive 
	them through the merge.`[1:],
		}
		auth = hivesim.Suite{
			Name: "auth",
			Description: `
	Test Engine API authentication features.`[1:],
		}
		sync = hivesim.Suite{
			Name: "sync",
			Description: `
	Test Engine API sync, pre/post merge.`[1:],
		}
	)

	simulator := hivesim.New()

	addTestsToSuite(&engine, engineTests)
	addTestsToSuite(&transition, mergeTests)
	addTestsToSuite(&auth, authTests)
	addSyncTestsToSuite(simulator, &sync, syncTests)

	// Mark suites for execution
	hivesim.MustRunSuite(simulator, engine)
	hivesim.MustRunSuite(simulator, transition)
	hivesim.MustRunSuite(simulator, auth)
	hivesim.MustRunSuite(simulator, sync)
}

// Add test cases to a given test suite
func addTestsToSuite(suite *hivesim.Suite, tests []TestSpec) {
	for _, currentTest := range tests {
		currentTest := currentTest
		genesisPath := "./init/genesis.json"
		// If the TestSpec specified a custom genesis file, use that instead.
		if currentTest.GenesisFile != "" {
			genesisPath = "./init/" + currentTest.GenesisFile
		}
		testFiles := hivesim.Params{"/genesis.json": genesisPath}
		// Calculate and set the TTD for this test
		ttd := calcRealTTD(genesisPath, currentTest.TTD)
		newParams := clientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
		if currentTest.ChainFile != "" {
			// We are using a Proof of Work chain file, remove all clique-related settings
			// TODO: Nethermind still requires HIVE_MINER for the Engine API
			// delete(newParams, "HIVE_MINER")
			delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
			delete(newParams, "HIVE_CLIQUE_PERIOD")
			// Add the new file to be loaded as chain.rlp
			testFiles = testFiles.Set("/chain.rlp", "./chains/"+currentTest.ChainFile)
		}
		suite.Add(hivesim.ClientTestSpec{
			Name:        currentTest.Name,
			Description: currentTest.About,
			Parameters:  newParams,
			Files:       testFiles,
			Run: func(t *hivesim.T, c *hivesim.Client) {
				t.Logf("Start test (%s): %s", c.Type, currentTest.Name)
				defer func() {
					t.Logf("End test (%s): %s", c.Type, currentTest.Name)
				}()
				timeout := DefaultTestCaseTimeout
				// If a TestSpec specifies a timeout, use that instead
				if currentTest.TimeoutSeconds != 0 {
					timeout = time.Second * time.Duration(currentTest.TimeoutSeconds)
				}
				// Run the test case
				RunTest(currentTest.Name, big.NewInt(ttd), currentTest.SlotsToSafe, currentTest.SlotsToFinalized, timeout, t, c, currentTest.Run, newParams, testFiles)
			},
		})
	}
}

// TTD is the value specified in the TestSpec + Genesis.Difficulty
func calcRealTTD(genesisPath string, ttdValue int64) int64 {
	g := loadGenesis(genesisPath)
	return g.Difficulty.Int64() + ttdValue
}
