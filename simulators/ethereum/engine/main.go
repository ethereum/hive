package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

	suite_auth "github.com/ethereum/hive/simulators/ethereum/engine/suites/auth"
	suite_engine "github.com/ethereum/hive/simulators/ethereum/engine/suites/engine"
	suite_sync "github.com/ethereum/hive/simulators/ethereum/engine/suites/sync"
	suite_transition "github.com/ethereum/hive/simulators/ethereum/engine/suites/transition"
)

func main() {
	var (
		engine = hivesim.Suite{
			Name: "engine-api",
			Description: `
	Test Engine API tests using CL mocker to inject commands into clients after they 
	have reached the Terminal Total Difficulty.`[1:],
		}
		transition = hivesim.Suite{
			Name: "engine-transition",
			Description: `
	Test Engine API tests using CL mocker to inject commands into clients and drive 
	them through the merge.`[1:],
		}
		auth = hivesim.Suite{
			Name: "engine-auth",
			Description: `
	Test Engine API authentication features.`[1:],
		}
		sync = hivesim.Suite{
			Name: "engine-sync",
			Description: `
	Test Engine API sync, pre/post merge.`[1:],
		}
	)

	simulator := hivesim.New()

	addTestsToSuite(&engine, suite_engine.Tests)
	addTestsToSuite(&transition, suite_transition.Tests)
	addTestsToSuite(&auth, suite_auth.Tests)
	suite_sync.AddSyncTestsToSuite(simulator, &sync, suite_sync.Tests)

	// Mark suites for execution
	hivesim.MustRunSuite(simulator, engine)
	hivesim.MustRunSuite(simulator, transition)
	hivesim.MustRunSuite(simulator, auth)
	hivesim.MustRunSuite(simulator, sync)
}

// Add test cases to a given test suite
func addTestsToSuite(suite *hivesim.Suite, tests []test.Spec) {
	for _, currentTest := range tests {
		currentTest := currentTest
		genesisPath := "./init/genesis.json"
		// If the test.Spec specified a custom genesis file, use that instead.
		if currentTest.GenesisFile != "" {
			genesisPath = "./init/" + currentTest.GenesisFile
		}
		testFiles := hivesim.Params{"/genesis.json": genesisPath}
		// Calculate and set the TTD for this test
		ttd := helper.CalculateRealTTD(genesisPath, currentTest.TTD)
		newParams := globals.DefaultClientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
		if currentTest.ChainFile != "" {
			// We are using a Proof of Work chain file, remove all clique-related settings
			// TODO: Nethermind still requires HIVE_MINER for the Engine API
			// delete(newParams, "HIVE_MINER")
			delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
			delete(newParams, "HIVE_CLIQUE_PERIOD")
			// Add the new file to be loaded as chain.rlp
			testFiles = testFiles.Set("/chain.rlp", "./chains/"+currentTest.ChainFile)
		}
		if currentTest.DisableMining {
			delete(newParams, "HIVE_MINER")
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
				timeout := globals.DefaultTestCaseTimeout
				// If a test.Spec specifies a timeout, use that instead
				if currentTest.TimeoutSeconds != 0 {
					timeout = time.Second * time.Duration(currentTest.TimeoutSeconds)
				}
				// Run the test case
				test.Run(currentTest.Name, big.NewInt(ttd), currentTest.SlotsToSafe, currentTest.SlotsToFinalized, timeout, t, c, currentTest.Run, newParams, testFiles, currentTest.TestTransactionType, currentTest.SafeSlotsToImportOptimistically)
			},
		})
	}
}
