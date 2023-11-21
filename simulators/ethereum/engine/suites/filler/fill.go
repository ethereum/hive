package filler

import (
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

type NodeType string

const (
	FullNode NodeType = "full"
)

func FillSuite(suite *hivesim.Suite, tests []test.Spec, nodeType NodeType) {
	runnerFunc := func(t *hivesim.T) {
		parallelism := 16
		if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
			if p, err := strconv.Atoi(val); err != nil {
				t.Logf("Warning: invalid HIVE_PARALLELISM value %q", val)
			} else {
				parallelism = p
			}
		}
		t.Log("parallelism", parallelism)

		random_seed := time.Now().Unix()
		if val, ok := os.LookupEnv("HIVE_RANDOM_SEED"); ok {
			if p, err := strconv.Atoi(val); err != nil {
				t.Logf("Warning: invalid HIVE_RANDOM_SEED value %q", val)
			} else if p > 0 {
				random_seed = int64(p)
			}
		}
		t.Log("random_seed", random_seed)

		var wg sync.WaitGroup
		var testCh = make(chan hivesim.TestSpec)
		wg.Add(parallelism)
		for i := 0; i < parallelism; i++ {
			go func() {
				defer wg.Done()
				for test := range testCh {
					t.Run(test)
				}
			}()
		}

		for _, currentTest := range tests {
			currentTest := currentTest
			currentTestName := fmt.Sprintf("%s (%s)", currentTest.GetName(), currentTest.GetMainFork())
			// Load the genesis file specified and dynamically bundle it.
			genesis := currentTest.GetGenesis()
			forkConfig := currentTest.GetForkConfig()
			if forkConfig == nil {
				// Test cannot be configured as is for current fork, skip
				fmt.Printf("skipping test \"%s\" because fork configuration is not possible\n", currentTestName)
				continue
			}
			forkConfig.ConfigGenesis(genesis)
			genesisStartOption, err := helper.GenesisStartOption(genesis)
			if err != nil {
				panic("unable to inject genesis")
			}

			// Calculate and set the TTD for this test
			ttd := genesis.Config.TerminalTotalDifficulty

			// Configure Forks
			newParams := globals.DefaultClientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", ttd))
			if forkConfig.LondonNumber != nil {
				newParams = newParams.Set("HIVE_FORK_LONDON", fmt.Sprintf("%d", forkConfig.LondonNumber))
			}
			if forkConfig.ParisNumber != nil {
				newParams = newParams.Set("HIVE_MERGE_BLOCK_ID", fmt.Sprintf("%d", forkConfig.ParisNumber))
			}
			if forkConfig.ShanghaiTimestamp != nil {
				newParams = newParams.Set("HIVE_SHANGHAI_TIMESTAMP", fmt.Sprintf("%d", forkConfig.ShanghaiTimestamp))
				// Ensure merge transition is activated before shanghai if not already
				if forkConfig.ParisNumber == nil {
					newParams = newParams.Set("HIVE_MERGE_BLOCK_ID", "0")
				}
				if forkConfig.CancunTimestamp != nil {
					newParams = newParams.Set("HIVE_CANCUN_TIMESTAMP", fmt.Sprintf("%d", forkConfig.CancunTimestamp))
				}
			}

			if nodeType != "" {
				newParams = newParams.Set("HIVE_NODETYPE", string(nodeType))
			}

			testFiles := hivesim.Params{}
			if genesis.Difficulty.Cmp(ttd) < 0 {
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
			} else {
				// This is a post-merge test
				delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
				delete(newParams, "HIVE_CLIQUE_PERIOD")
				delete(newParams, "HIVE_MINER")
				newParams = newParams.Set("HIVE_POST_MERGE_GENESIS", "true")
			}

			if clientTypes, err := t.Sim.ClientTypes(); err == nil {
				for _, clientType := range clientTypes {
					test := hivesim.TestSpec{
						Name:        fmt.Sprintf("%s (%s)", currentTestName, clientType.Name),
						Description: currentTest.GetDescription(),
						Category:    currentTest.GetCategory(),
						Run: func(t *hivesim.T) {
							// Start the client with given options
							c := t.StartClient(
								clientType.Name,
								newParams,
								genesisStartOption,
								hivesim.WithStaticFiles(testFiles),
							)
							t.Logf("Start test (%s): %s", c.Type, currentTestName)
							defer func() {
								t.Logf("End test (%s): %s", c.Type, currentTestName)
							}()
							timeout := globals.DefaultTestCaseTimeout
							// If a test.Spec specifies a timeout, use that instead
							if currentTest.GetTimeout() != 0 {
								timeout = time.Second * time.Duration(currentTest.GetTimeout())
							}
							// Run the test case
							test.Run(
								currentTest,
								new(big.Int).Set(ttd),
								timeout,
								t,
								c,
								genesis,
								rand.New(rand.NewSource(random_seed)),
								newParams,
								testFiles,
							)
						},
					}
					testCh <- test
				}
			}
		}

		// Wait for all test threads to finish.
		close(testCh)
		defer wg.Wait()
	}

	suite.Add(hivesim.TestSpec{
		Name:        fmt.Sprintf("%s test loader", suite.Name),
		Description: "",
		Run:         runnerFunc,
		AlwaysRun:   true,
	})
}
