// rpc-compat-v2 is like rpc-compat but designed to be more easily extensible with different test modes / finer granularity testing.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ethereum/hive/hivesim"
)

func main() {
	// ensure ./testcases folder exists
	relPath := filepath.Join(".", "tests")
	testcasesFolderExists := FolderExistsAtRelativePath(relPath)
	if !testcasesFolderExists {
		panic("./tests folder does not exist")
	}

	// load fork env
	var clientEnv hivesim.Params
	forkenvFilepath := filepath.Join(".", "tests", "forkenv.json")
	err := LoadJSON(forkenvFilepath, &clientEnv)
	if err != nil {
		panic(err)
	}

	// run test suite
	suite := hivesim.Suite{
		Name: "rpc-compat-v2",
		Description: `
The RPC-compatibility test suite runs a set of RPC related tests against a
running node. It tests client implementations of the JSON-RPC API for
conformance with the execution API specification.`[1:],
	}
	suite.Add(&hivesim.ClientTestSpec{
		Role:        "eth1",
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Parameters:  clientEnv,
		Files:       GenesisJsonAndChainRlpFileMapping,
		Run: func(t *hivesim.T, c *hivesim.Client) {
			sendForkchoiceUpdated(t, c)
			runAllTests(t, c, c.Type)
		},
		AlwaysRun: true,
	})
	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)

}

func sendRPC(rpc RPC, url string, verbose bool) (RPCResp, error) {
	// define req
	body, _ := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  rpc.Method,
		"params":  rpc.Params,
		"id":      1,
	})
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// send req
	client := &http.Client{Timeout: CLIENT_TIMEOUT}
	log.Printf("Will now send RPC %v..\n", rpc.Method)
	res, err := client.Do(req)
	if err != nil {
		return RPCResp{}, err
	}
	defer res.Body.Close()

	// parse resp
	var out RPCResp
	if err := json.NewDecoder(res.Body).Decode(&out); err != nil {
		return RPCResp{}, err
	}
	// log entire response in verbose mode TODO: use hive ll instead
	if verbose {
		out.Dump()
	}
	if out.Error != nil {
		return RPCResp{}, errors.New(out.Error.GetDump())
	}
	if out.Result == nil {
		return RPCResp{}, fmt.Errorf("no result")
	}

	return out, nil
}

func runAllTests(t *hivesim.T, c *hivesim.Client, clientName string) {
	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	relPath := filepath.Join(".", "tests")
	tests := loadTests(t, relPath, "io", re)
	for _, test := range tests {
		test := test // maybe not needed anymore
		t.Run(hivesim.TestSpec{
			Name: fmt.Sprintf("%s (%s)", test.Filepath, clientName),
			// leaving Description empty for now
			Run: func(t *hivesim.T) {
				resp, err := runTest(t, c, &test)
				if err != nil {
					t.Fatal(err)
				}

				// given the response data and the test (e.g. which along other info contains a test type like EXACT MATCH), determine TestResult (c.test.result)
				testResult := determineTestResult(t, resp, test)
				if !testResult.Pass {
					t.Fail()
				}
				//t.Pass() does not exist afaik, i think in hivesim a test passing is the default and if it failed you call Fail() but if it passed u do nothing
			},
		})
	}
}

// runTest only runs the test, then collects and returns the response and any error.
// The passed/failed decision logic has been moved into its own function.
func runTest(t *hivesim.T, c *hivesim.Client, test *RPCTestcase) (RPCResp, error) {
	_ = t // TODO: unused, but i want to set verbose via t.Sim.ll (but i need a getter cuz unexported?)
	url := fmt.Sprintf("http://%s", net.JoinHostPort(c.IP.String(), CLIENT_PORT))

	verbose := true
	resp, err := sendRPC(test.Input, url, verbose)
	return resp, err
}

func determineTestResult(t *hivesim.T, actualResp RPCResp, testObject RPCTestcase) hivesim.TestResult {
	var testResult hivesim.TestResult

	printSeparator()
	t.Logf("Will use test mode %v for %v fail/pass logic", testObject.TestMode, testObject.Input.Method)

	// currently there are 3 test types (EXACT, RANGE, TYPE) which determine the criteria for the pass/fail decision
	switch testObject.TestMode.(type) {
	case TestExactMatch, *TestExactMatch:
		testResult = determineTestResultExactMatch(t, actualResp, testObject)
	case TestRangedMatch, *TestRangedMatch:
		testResult = determineTestResultRangedMatch(t, actualResp, testObject)
	case TestTypeMatch, *TestTypeMatch: // old name: specsonly
		testResult = determineTestResultTypeMatch(t, actualResp, testObject)
	default:
		panic("Unknown test type")
	}

	return testResult
}

// sendForkchoiceUpdated delivers the initial FcU request to the client.
func sendForkchoiceUpdated(t *hivesim.T, client *hivesim.Client) {
	var request struct {
		Method string
		Params []any
	}
	headfcuFilepath := filepath.Join(".", "tests", "headfcu.json")
	if err := LoadJSON(headfcuFilepath, &request); err != nil {
		t.Fatal("error loading forkchoiceUpdated:", err)
	}
	t.Logf("sending %s: %v", request.Method, request.Params)
	var resp any
	err := client.EngineAPI().Call(&resp, request.Method, request.Params...)
	if err != nil {
		t.Fatal("client rejected forkchoiceUpdated:", err)
	}
	t.Logf("response: %v", resp)
}

func printSeparator() {
	fmt.Println(strings.Repeat("-", 120))
}

// loadTests finds and parses io files in any subdir, returns test cases loaded from these files
func loadTests(t *hivesim.T, relPath string, file_extension string, re *regexp.Regexp) []RPCTestcase {
	testCases := []RPCTestcase{}

	ioFileList := GetFilesInFolderRecursively(relPath, file_extension, re)
	for _, f := range ioFileList {
		// parse io tests one-by-one
		t.Logf("Reading %v\n", f)
		fileContent := ReadFileAtRelativePath(f)
		req, resp_expect := ParseIoContent(t, fileContent)

		req.Dump()
		resp_expect.Dump()

		testCase := RPCTestcase{
			Filepath: f,
			Input:    req,
			// ExpectedOutput:	 will be populated later (depends on resp_expect and TestMode)
			// TestMode:		 will be populated next
		}
		// TODO: depending on which RPC this is, decide which testcase should be chosen for evaluating response correctness
		// e.g. req.TestTypeMapping() // looks at req.Method and populates

		// TODO: below is just for be able to run this for now
		testCase.ExpectedOutput = resp_expect
		testCase.TestMode = TestExactMatch{}

		testCases = append(testCases, testCase)

	}

	return testCases

}

// v2 rpc compat geth fails 48 tests, v1 rpc compat geth fails 49 tests. figure out which test is judged differently and why
// TODO: in loadTests actually call a mapper so that it does not just use TestExactMatch for everything
// TODO: implement all decision logic in pass-fail-logic.go
// TODO: testing - check whether regex inclusion/exclusion still works as in v1
// TODO: testing - adjust docker file to ensure that case where ./tests is missing still works
// TODO: testing - test this with multiple clients at once
// TODO: respect hive loglevel (ll) instead of putting verbose true/false
// optional TODO: add function to generate io expectation files for all RPCs with any running client
// optional TODO: add performance measuring (test start time, test end time) by populating values in test type object
// optional non-technical work: find agreement for each RPC that uses ranged test type so that valid value ranges can be agreed upon. or just make the io generation dynamic and create more testchains and scold every client that fails anything lul
