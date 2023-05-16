// ============================================================================== //
// Pyspec Hive Simulator: Ported/Altered directly from the Consensus Simulator.   //
// -> https://github.com/ethereum/hive/tree/master/simulators/ethereum/consensus  //
// ============================================================================== //

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/hive/hivesim"
)

func main() {
	suite := hivesim.Suite{
		Name: "pyspec",
		Description: "The 'pyspec' test suite runs every test fixture from " +
			"the execution-spec-tests repository (https://github.com/ethereum/execution-spec-tests)" +
			"against each client specified in the hive simulation run for forks >= Merge. " +
			"The clients are first fed a fixture genesis field, followed by each fixture block. " +
			"The last valid block is then queried for its storage, nonce & balance, that are compared" +
			"against the expected values from the test fixture file. This is all achieved using the EngineAPI.",
	}
	suite.Add(hivesim.TestSpec{
		Name: "pytest_fixture_runner",
		Description: "This is a meta-test. It loads the test fixture files and " +
			"launches the actual client tests. Any errors in test files will be reported " +
			"through this test.",
		Run:       fixtureRunner,
		AlwaysRun: true,
	})
	hivesim.MustRunSuite(hivesim.New(), suite)
}

// fixtureRunner loads the pyspec test files and spawns the client tests.
func fixtureRunner(t *hivesim.T) {

	// retrieve clients available for testing
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatal("can't get client types:", err)
	}

	// use 16 for parallelism if env var is invalid
	parallelism := 16
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			t.Logf("warning: invalid HIVE_PARALLELISM value %q", val)
		} else {
			parallelism = p
		}
	}
	t.Log("parallelism set to:", parallelism)

	// find and set the fixtures directory as root
	testPath, isset := os.LookupEnv("TESTPATH")
	if !isset {
		t.Fatal("$TESTPATH not set")
	}
	fileRoot := fmt.Sprintf("%s/", testPath)
	t.Log("file root directory:", fileRoot)

	// to log all failing tests at the end of sim
	failedTests := make(map[string]error)

	// spawn `parallelism` workers to run fixtures against clients
	var wg sync.WaitGroup
	var testCh = make(chan *testcase)
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			for test := range testCh {
				t.Run(hivesim.TestSpec{
					Name: test.name,
					Description: ("Test Link: " +
						repoLink(test.filepath)),
					Run:       test.run,
					AlwaysRun: false,
				})
				if test.failedErr != nil {
					failedTests[test.clientType+"/"+test.name] = test.failedErr
				}
			}
		}()
	}

	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	// deliver and run test cases against each client
	loadFixtureTests(t, fileRoot, re, func(tc testcase) {
		for _, client := range clientTypes {
			if !client.HasRole("eth1") {
				continue
			}
			tc := tc // shallow copy
			tc.clientType = client.Name
			testCh <- &tc
		}
	})
	close(testCh)

	// wait for all workers to finish
	wg.Wait()

	// log all failed tests
	if len(failedTests) > 0 {
		t.Log("failing tests:")
		for name, err := range failedTests {
			t.Logf("%v: %v", name, err)
		}
	}
}

// repoLink coverts a pyspec test path into a github repository link.
func repoLink(testPath string) string {
	// Example for withdrawals_zero_amout.json:
	// Converts '/fixtures/withdrawals/withdrawals/withdrawals_zero_amount.json'
	// into 'fillers/withdrawals/withdrawals.py',
	// and appends onto main branch repo link.
	filePath := strings.Replace(testPath, "/fixtures", "fillers", -1)
	fileDir := strings.TrimSuffix(filePath, "/"+filepath.Base(filePath)) + ".py"
	repoLink := fmt.Sprintf(
		"https://github.com/ethereum/execution-spec-tests/blob/main/%v",
		fileDir)
	return repoLink
}
