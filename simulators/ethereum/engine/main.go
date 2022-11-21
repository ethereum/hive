package main

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

	suite_withdrawals "github.com/ethereum/hive/simulators/ethereum/engine/suites/withdrawals"
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
		withdrawals = hivesim.Suite{
			Name: "engine-withdrawals",
			Description: `
	Test Engine API withdrawals, pre/post Shanghai.`[1:],
		}
	)

	simulator := hivesim.New()

	//addTestsToSuite(&engine, suite_engine.Tests, "full")
	//addTestsToSuite(&transition, suite_transition.Tests, "full")
	//addTestsToSuite(&auth, suite_auth.Tests, "full")
	//suite_sync.AddSyncTestsToSuite(simulator, &sync, suite_sync.Tests)
	addTestsToSuite(&withdrawals, suite_withdrawals.Tests, "full")

	// Mark suites for execution
	hivesim.MustRunSuite(simulator, engine)
	hivesim.MustRunSuite(simulator, transition)
	hivesim.MustRunSuite(simulator, auth)
	hivesim.MustRunSuite(simulator, sync)
	hivesim.MustRunSuite(simulator, withdrawals)
}

// Add test cases to a given test suite
func addTestsToSuite(suite *hivesim.Suite, tests []test.SpecInterface, nodeType string) {
	for _, currentTest := range tests {
		currentTest := currentTest
		genesisPath := "./init/genesis.json"
		// If the test.Spec specified a custom genesis file, use that instead.
		if currentTest.GetGenesisFile() != "" {
			genesisPath = "./init/" + currentTest.GetGenesisFile()
		}
		// Load genesis for it to be modified before starting the client
		testFiles := hivesim.Params{"/genesis.json": genesisPath}
		// Calculate and set the TTD for this test
		ttd := helper.CalculateRealTTD(genesisPath, currentTest.GetTTD())
		// Configure Forks
		newParams := globals.DefaultClientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
		if currentTest.GetForkConfig().ShanghaiTimestamp != nil {
			newParams = newParams.Set("HIVE_SHANGHAI_TIMESTAMP", fmt.Sprintf("%d", currentTest.GetForkConfig().ShanghaiTimestamp))
		}

		if nodeType != "" {
			newParams = newParams.Set("HIVE_NODETYPE", nodeType)
		}

		if currentTest.GetChainFile() != "" {
			// We are using a Proof of Work chain file, remove all clique-related settings
			// TODO: Nethermind still requires HIVE_MINER for the Engine API
			// delete(newParams, "HIVE_MINER")
			delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
			delete(newParams, "HIVE_CLIQUE_PERIOD")
			// Add the new file to be loaded as chain.rlp
			testFiles = testFiles.Set("/chain.rlp", "./chains/"+currentTest.GetChainFile())
		}
		if currentTest.IsMiningDisabled() {
			delete(newParams, "HIVE_MINER")
		}
		suite.Add(hivesim.ClientTestSpec{
			Name:        currentTest.GetName(),
			Description: currentTest.GetAbout(),
			Parameters:  newParams,
			Files:       testFiles,
			Run: func(t *hivesim.T, c *hivesim.Client) {
				t.Logf("Start test (%s): %s", c.Type, currentTest.GetName())
				defer func() {
					t.Logf("End test (%s): %s", c.Type, currentTest.GetName())
				}()
				timeout := globals.DefaultTestCaseTimeout
				// If a test.Spec specifies a timeout, use that instead
				if currentTest.GetTimeout() != 0 {
					timeout = time.Second * time.Duration(currentTest.GetTimeout())
				}
				// Run the test case
				test.Run(currentTest, big.NewInt(ttd), timeout, t, c, newParams, testFiles)
			},
		})
	}
}
