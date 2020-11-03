package common

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IndexFileName is the main index file of the test suite execution database
const IndexFileName string = "index.txt"

// TestSuiteID identifies a test suite context
type TestSuiteID uint32

func (tsID TestSuiteID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestID identifies a test case context
type TestID uint32

func (tsID TestID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestSummary is an entry in a main index of test suite executions
// NB! This record must be small (less than the typically smallest file sector size)
// to ensure cross-platform atomic writes to the index file from multiple processes.
type TestSummary struct {
	FileName      string    `json:"fileName"`
	Name          string    `json:"name"`
	Start         time.Time `json:"start"`
	PrimaryClient string    `json:"primaryClient"`
	Pass          bool      `json:"pass"`
	// the log-file pertaining to the simulator. (may encompass more than just one TestSuite)
	SimulatorLog string `json:"simLog"`
}

// TestSuite is a single run of a simulator, a collection of testcases
type TestSuite struct {
	ID          TestSuiteID          `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TestCases   map[TestID]*TestCase `json:"testCases"`
	// the log-file pertaining to the simulator. (may encompass more than just one TestSuite)
	SimulatorLog string `json:"simLog"`
	indexMu      sync.Mutex
}

func (testSuite *TestSuite) summarise(suiteFileName string) TestSummary {

	summary := TestSummary{
		FileName:     suiteFileName,
		Name:         testSuite.Name,
		SimulatorLog: testSuite.SimulatorLog,
	}

	pass := true
	earliest := time.Now()
	clients := make(map[string]bool, 0)
	for _, testCase := range testSuite.TestCases {
		pass = pass && testCase.SummaryResult.Pass
		if testCase.Start.Before(earliest) {
			earliest = testCase.Start
		}
		for _, clientInfo := range testCase.ClientInfo {
			clients[clientInfo.Name] = true
		}
	}
	summary.Pass = pass
	summary.Start = earliest

	var clientNames []string
	for k, _ := range clients {
		clientNames = append(clientNames, k)
	}
	if len(clientNames) > 0 {
		summary.PrimaryClient = strings.Join(clientNames, ",")
	} else {
		summary.PrimaryClient = "None"
	}
	return summary
}

// UpdateDB should be called on TestSuite completion to write out the test results and update indexes.
func (testSuite *TestSuite) UpdateDB(outputPath string) error {

	var (
		suiteFileName          string                                     // The name of a json file with test data
		indexPathName          = filepath.Join(outputPath, IndexFileName) // Path to the global index file, listing all files
		summaryData, suiteData []byte
		err                    error
	)

	// write out the test suite as a json file of its own
	if suiteData, err = json.Marshal(*testSuite); err != nil {
		return err
	}

	{
		// Randomize the name, but make it so that it's ordered by date - makes cleanups easier
		b := make([]byte, 16)
		rand.Read(b)
		suiteFileName = fmt.Sprintf("%v-%x.json", time.Now().Unix(), b)
	}
	// marshall the summary
	if summaryData, err = json.Marshal(testSuite.summarise(suiteFileName)); err != nil {
		return err
	}
	// write the file
	ioutil.WriteFile(filepath.Join(outputPath, suiteFileName), suiteData, 0644)

	// Before writing the summary, perhaps crop the index file
	// We cap it at 400KB in size down to 200KB, roughly 1000 lines
	testSuite.indexMu.Lock()
	defer testSuite.indexMu.Unlock()
	stat, err := os.Stat(indexPathName)
	if err == nil && stat.Size() > 400000 {
		truncateHead(indexPathName, 200000)
	}

	//now append the index file
	return appendLine(indexPathName, summaryData)
}

// TestClientResults is the set of per-client results for a test-case
type TestClientResults map[string]*TestResult

// AddResult adds a test result for a client
func (t TestClientResults) AddResult(clientID string, pass bool, detail string) {
	tcr, in := t[clientID]
	if !in {
		tcr = &TestResult{}
		t[clientID] = tcr
	}
	tcr.Pass = pass
	tcr.AddDetail(detail)
}

// TestCase represents a single test case in a test suite
type TestCase struct {
	ID            TestID                     `json:"id"`          // Test case reference number.
	Name          string                     `json:"name"`        // Test case short name.
	Description   string                     `json:"description"` // Test case long description in MD.
	Start         time.Time                  `json:"start"`
	End           time.Time                  `json:"end"`
	SummaryResult TestResult                 `json:"summaryResult"` // The result of the whole test case.
	ClientResults TestClientResults          `json:"clientResults"` // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
	ClientInfo    map[string]*TestClientInfo `json:"clientInfo"`    // Info about each client.
}

// TestResult describes the results of a test at the level of the overall test case and for each client involved in a test case
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
}

// AddDetail adds test result info using a standard text formatting
func (t *TestResult) AddDetail(detail string) {
	t.Details = fmt.Sprintf("%s %s\n", t.Details, detail)
}

// TestClientInfo describes a client that participated in a test case
type TestClientInfo struct {
	ID              string    `json:"id"`
	IP              string    `json:"ip"`
	Name            string    `json:"name"`
	VersionInfo     string    `json:"versionInfo"` //URL to github repo + branch.
	InstantiatedAt  time.Time `json:"instantiatedAt"`
	LogFile         string    `json:"logFile"` //Absolute path to the logfile.
	WasInstantiated bool
}
