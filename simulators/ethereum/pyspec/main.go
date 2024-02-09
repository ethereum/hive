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
		Description: "The pyspec test suite runs every fixture from " +
			"the execution-spec-tests repo (https://github.com/ethereum/execution-spec-tests) where the fork >= Merge. " +
			"For each test clients are first fed the fixture genesis data followed by engine new payloads specific to the test.",
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
	var testCh = make(chan *TestCase)
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			for test := range testCh {
				t.Run(hivesim.TestSpec{
					Name: test.Name,
					Description: ("Test Link: " +
						repoLink(test.FilePath)),
					Run:       test.run,
					AlwaysRun: false,
				})
				if test.FailedErr != nil {
					failedTests[test.ClientType+"/"+test.Name] = test.FailedErr
				}
			}
		}()
	}

	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	// deliver and run test cases against each client
	loadFixtureTests(t, fileRoot, re, func(tc TestCase) {
		for _, client := range clientTypes {
			if !client.HasRole("eth1") {
				continue
			}
			tc := tc // shallow copy
			tc.ClientType = client.Name
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
	// Example: Converts '/fixtures/cancun/eip4844_blobs/blob_txs/invalid_normal_gas.json'
	// into 'tests/cancun/eip4844_blobs/test_blob_txs.py', and appends onto main branch repo link.
	filePath := strings.Replace(testPath, "/fixtures", "tests", -1)
	fileDir := filepath.Dir(filePath)
	fileBase := filepath.Base(fileDir)
	fileName := filepath.Join(filepath.Dir(fileDir), "test_"+fileBase+".py")
	repoLink := fmt.Sprintf("https://github.com/ethereum/execution-spec-tests/tree/main/%v", fileName)
	return repoLink
}
