package main

import (
	"fmt"
	"math/big"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/ethereum/engine/globals"
	"github.com/ethereum/hive/simulators/ethereum/engine/helper"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"

	//suite_auth "github.com/ethereum/hive/simulators/ethereum/engine/suites/auth"
	//suite_engine "github.com/ethereum/hive/simulators/ethereum/engine/suites/engine"
	//suite_ex_cap "github.com/ethereum/hive/simulators/ethereum/engine/suites/exchange_capabilities"
	//suite_transition "github.com/ethereum/hive/simulators/ethereum/engine/suites/transition"
	suite_cancun "github.com/ethereum/hive/simulators/ethereum/engine/suites/cancun"
	suite_withdrawals "github.com/ethereum/hive/simulators/ethereum/engine/suites/withdrawals"
)

// we will wait that time before every test execution to be sure environment is ready
// may be changed to other value if necessary
const SetupTime = 3 * time.Minute

func main() {
	var (
		//	engine = hivesim.Suite{
		//		Name: "engine-api",
		//		Description: `
		//Test Engine API tests using CL mocker to inject commands into clients after they
		//have reached the Terminal Total Difficulty.`[1:],
		//	}
		//	transition = hivesim.Suite{
		//		Name: "engine-transition",
		//		Description: `
		//Test Engine API tests using CL mocker to inject commands into clients and drive
		//them through the merge.`[1:],
		//	}
		//	auth = hivesim.Suite{
		//		Name: "engine-auth",
		//		Description: `
		//Test Engine API authentication features.`[1:],
		//	}
		//	excap = hivesim.Suite{
		//		Name: "engine-exchange-capabilities",
		//		Description: `
		//Test Engine API exchange capabilities.`[1:],
		//	}
		//	sync = hivesim.Suite{
		//		Name: "engine-sync",
		//		Description: `
		//Test Engine API sync, pre/post merge.`[1:],
		//	}
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
	simulator := hivesim.New()

	//addTestsToSuite(simulator, &engine, specToInterface(suite_engine.Tests), "full")
	//addTestsToSuite(simulator, &transition, specToInterface(suite_transition.Tests), "full")
	//addTestsToSuite(simulator, &auth, specToInterface(suite_auth.Tests), "full")
	//addTestsToSuite(simulator, &excap, specToInterface(suite_ex_cap.Tests), "full")
	//suite_sync.AddSyncTestsToSuite(simulator, &sync, suite_sync.Tests)
	addTestsToSuite(simulator, &withdrawals, suite_withdrawals.Tests, "full")
	addTestsToSuite(simulator, &cancun, suite_cancun.Tests, "full")

	// Mark suites for execution
	//hivesim.MustRunSuite(simulator, engine)
	//hivesim.MustRunSuite(simulator, transition)
	//hivesim.MustRunSuite(simulator, auth)
	//hivesim.MustRunSuite(simulator, excap)
	// hivesim.MustRunSuite(simulator, syncSuite)
	hivesim.MustRunSuite(simulator, withdrawals)
	hivesim.MustRunSuite(simulator, cancun)
}

//	func specToInterface(src []test.Spec) []test.SpecInterface {
//		res := make([]test.SpecInterface, len(src))
//		for i := 0; i < len(src); i++ {
//			res[i] = src[i]
//		}
//		return res
//	}
type ClientGenesis interface {
	GetTTD()
}

func getTimestamp(spec test.Spec) int64 {
	now := time.Now()

	preShapellaBlock := spec.GetPreShapellaBlockCount()
	if preShapellaBlock == 0 {
		preShapellaBlock = 1
	}

	preShapellaTime := time.Duration(uint64(preShapellaBlock)*spec.GetBlockTimeIncrements()) * time.Second
	// after setup wait chain will produce blocks in preShapellaTime and than shapella happens
	shanghaiTimestamp := now.Add(SetupTime).Add(preShapellaTime)
	return shanghaiTimestamp.Unix()
}

// Add test cases to a given test suite
func addTestsToSuite(sim *hivesim.Simulation, suite *hivesim.Suite, tests []test.Spec, nodeType string) {
	for _, currentTest := range tests {
		currentTest := currentTest

		// Load the genesis file specified and dynamically bundle it.
		clientTypes, err := sim.ClientTypes()
		if err != nil {
			return
		}
		if len(clientTypes) != 1 {
			panic("missing client")
		}
		clientName := strings.Split(clientTypes[0].Name, "_")[0]
		random_seed := time.Now().Unix()
		genesis := currentTest.GetGenesis(clientName)

		// Set the timestamp of the genesis to the next 2 minutes
		timestamp := getTimestamp(currentTest)
		genesis.SetTimestamp(timestamp)
		genesis.SetDifficulty(big.NewInt(100))
		//genesis.UpdateTimestamp(getTimestamp())
		genesisStartOption, err := helper.GenesisStartOptionBasedOnClient(genesis, clientName)
		if err != nil {
			panic("unable to inject genesis")
		}
		forkConfig := currentTest.GetForkConfig()
		//forkConfig.ConfigGenesisHelper(genesis)
		// Calculate and set the TTD for this test
		//ttd := helper.CalculateulateRealTTD(genesis, currentTest.GetTTD())

		// Configure Forks
		newParams := globals.DefaultClientEnv.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", fmt.Sprintf("%d", genesis.Difficulty().Uint64()))
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
			newParams = newParams.Set("HIVE_NODETYPE", nodeType)
		}

		testFiles := hivesim.Params{}
		//if genesis.Difficulty().Cmp(big.NewInt(ttd)) < 0 {
		//
		//	if currentTest.GetChainFile() != "" {
		//		// We are using a Proof of Work chain file, remove all clique-related settings
		//		// TODO: Nethermind still requires HIVE_MINER for the Engine API
		//		// delete(newParams, "HIVE_MINER")
		//		delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
		//		delete(newParams, "HIVE_CLIQUE_PERIOD")
		//		// Add the new file to be loaded as chain.rlp
		//		testFiles = testFiles.Set("/chain.rlp", "./chains/"+currentTest.GetChainFile())
		//	}
		//	if currentTest.IsMiningDisabled() {
		//		delete(newParams, "HIVE_MINER")
		//	}
		//} else {
		// This is a post-merge test
		delete(newParams, "HIVE_CLIQUE_PRIVATEKEY")
		delete(newParams, "HIVE_CLIQUE_PERIOD")
		delete(newParams, "HIVE_MINER")
		newParams = newParams.Set("HIVE_POST_MERGE_GENESIS", "false")
		newParams = newParams.Set("HIVE_NETWORK_ID", strconv.FormatInt(genesis.Config().ChainID.Int64(), 10))
		t := strconv.FormatUint(genesis.Difficulty().Uint64(), 10)
		newParams = newParams.Set("HIVE_TERMINAL_TOTAL_DIFFICULTY", t)
		//}

		if clientTypes, err := sim.ClientTypes(); err == nil {
			for _, clientType := range clientTypes {
				suite.Add(hivesim.TestSpec{
					Name:        fmt.Sprintf("%s (%s)", currentTest.GetName(), clientType.Name),
					Description: currentTest.GetAbout(),
					Run: func(t *hivesim.T) {
						// Start the client with given options
						c := t.StartClient(
							clientType.Name,
							newParams,
							genesisStartOption,
							hivesim.WithStaticFiles(testFiles),
						)
						t.Logf("Start test (%s): %s", c.Type, currentTest.GetName())
						defer func() {
							t.Logf("End test (%s): %s", c.Type, currentTest.GetName())
						}()
						currentTest.GetName()
						timeout := 30 * time.Minute
						// If a test.Spec specifies a timeout, use that instead
						if currentTest.GetTimeout() != 0 {
							timeout = time.Second * time.Duration(currentTest.GetTimeout())
						}
						time.Sleep(30 * time.Second)
						// Run the test case
						test.Run(
							currentTest,
							genesis.Difficulty(),
							timeout,
							t,
							c,
							genesis,
							rand.New(rand.NewSource(random_seed)), newParams,
							testFiles,
						)
					},
				})
			}
		}

	}
}
