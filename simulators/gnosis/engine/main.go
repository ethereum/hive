package main

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
	suite_auth "github.com/ethereum/hive/simulators/ethereum/engine/suites/auth"
	suite_cancun "github.com/ethereum/hive/simulators/ethereum/engine/suites/cancun"
	suite_engine "github.com/ethereum/hive/simulators/ethereum/engine/suites/engine"
	suite_excap "github.com/ethereum/hive/simulators/ethereum/engine/suites/exchange_capabilities"
	suite_withdrawals "github.com/ethereum/hive/simulators/ethereum/engine/suites/withdrawals"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

const setupTime = time.Minute

var (
	engine = hivesim.Suite{
		Name: "engine-api",
		Description: `
	Test Engine API tests using CL mocker to inject commands into clients after they 
	have reached the Terminal Total Difficulty.`[1:],
	}
	auth = hivesim.Suite{
		Name: "engine-auth",
		Description: `
	Test Engine API authentication features.`[1:],
	}
	excap = hivesim.Suite{
		Name: "engine-exchange-capabilities",
		Description: `
	Test Engine API exchange capabilities.`[1:],
	}
	withdrawals = hivesim.Suite{
		Name: "engine-withdrawals",
		Description: `
	Test Engine API withdrawals, pre/post Shanghai.`[1:],
	}
	cancun = hivesim.Suite{
		Name: "engine-cancun",
		Description: `
	Test Engine API on Cancun.`[1:],
	}
)

func main() {
	engine.Add(hivesim.TestSpec{
		Name:        "engine test loader",
		Description: "",
		Run:         makeRunner(suite_engine.Tests, "full"),
		AlwaysRun:   true,
	})
	auth.Add(hivesim.TestSpec{
		Name:        "engine-auth test loader",
		Description: "",
		Run:         makeRunner(suite_auth.Tests, "full"),
		AlwaysRun:   true,
	})
	excap.Add(hivesim.TestSpec{
		Name:        "engine-excap test loader",
		Description: "",
		Run:         makeRunner(suite_excap.Tests, "full"),
		AlwaysRun:   true,
	})
	withdrawals.Add(hivesim.TestSpec{
		Name:        "engine-withdrawals test loader",
		Description: "",
		Run:         makeRunner(suite_withdrawals.Tests, "full"),
		AlwaysRun:   true,
	})
	cancun.Add(hivesim.TestSpec{
		Name:        "engine-cancun test loader",
		Description: "",
		Run:         makeRunner(suite_cancun.Tests, "full"),
		AlwaysRun:   true,
	})
	simulator := hivesim.New()

	// Mark suites for execution
	hivesim.MustRunSuite(simulator, engine)
	hivesim.MustRunSuite(simulator, auth)
	hivesim.MustRunSuite(simulator, excap)
	hivesim.MustRunSuite(simulator, withdrawals)
	hivesim.MustRunSuite(simulator, cancun)
}

func getTimestamp(spec test.Spec) uint64 {
	now := time.Now()

	preShapellaBlock := spec.GetPreShapellaBlockCount()
	if preShapellaBlock == 0 {
		preShapellaBlock = 1
	}

	preShapellaTime := time.Duration(uint64(preShapellaBlock)*spec.GetBlockTimeIncrements()) * time.Second

	// After setup wait chain will produce blocks in preShapellaTime and then shapella happens.
	shanghaiTimestamp := now.Add(setupTime).Add(preShapellaTime)
	return uint64(shanghaiTimestamp.Unix())
}

func makeRunner(tests []test.Spec, nodeType string) func(t *hivesim.T) {
	return func(t *hivesim.T) {
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

			// Get the timestamp for the test
			timestamp := new(big.Int).SetUint64(getTimestamp(currentTest))
			if currentTest.GetMainFork() != "Cancun" {
				globals.GenesisTimestamp = timestamp.Uint64()
			}

			// Load the genesis file specified and dynamically bundle it.
			genesis := currentTest.GetGenesis()
			forkConfig := currentTest.GetForkConfig()
			if forkConfig == nil {
				// Test cannot be configured as is for current fork, skip
				fmt.Printf("skipping test \"%s\" because fork configuration is not possible\n", currentTestName)
				continue
			}

			// Set the timestamps for the forks
			shanghaiTimestamp := new(big.Int).Add(timestamp, big.NewInt(-1000000))
			cancunTimestamp := new(big.Int).Add(timestamp, big.NewInt(0))
			forkConfig.CancunTimestamp = cancunTimestamp

			if currentTest.GetMainFork() == "Cancun" {
				shanghaiTimestamp = forkConfig.CancunTimestamp
			}

			forkConfig.ShanghaiTimestamp = shanghaiTimestamp

			// Configure the genesis file
			forkConfig.ConfigGenesis(genesis)

			// Get the genesis start option
			genesisStartOption, err := helper.GenesisStartOption(genesis)
			if err != nil {
				panic("unable to inject genesis")
			}

			// Configure the parameters for the clients
			newParams := globals.DefaultClientEnv
			newParams = newParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY_PASSED", "1")
			newParams = newParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", genesis.Difficulty))
			newParams = newParams.Set("HIVE_MERGE_BLOCK_ID", "0")

			// Configure the timestamps for the forks
			if forkConfig.ShanghaiTimestamp != nil {
				newParams = newParams.Set("HIVE_SHANGHAI_TIMESTAMP", fmt.Sprintf("%d", forkConfig.ShanghaiTimestamp))
				if forkConfig.CancunTimestamp != nil {
					newParams = newParams.Set("HIVE_CANCUN_TIMESTAMP", fmt.Sprintf("%d", forkConfig.CancunTimestamp))
				}
			}

			if nodeType != "" {
				newParams = newParams.Set("HIVE_NODETYPE", nodeType)
			}

			if clientTypes, err := t.Sim.ClientTypes(); err == nil {
				for _, clientType := range clientTypes {
					test := hivesim.TestSpec{
						Name:        fmt.Sprintf("%s (%s)", currentTestName, clientType.Name),
						Description: currentTest.GetAbout(),
						Run: func(t *hivesim.T) {
							// Start the client with given options
							c := t.StartClient(
								clientType.Name,
								newParams,
								genesisStartOption,
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
								timeout,
								t,
								c,
								genesis,
								rand.New(rand.NewSource(random_seed)),
								newParams,
								hivesim.Params{},
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
}
