package libhive

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"time"
)

// SimEnv contains the simulation parameters.
type SimEnv struct {
	SimLogLevel          int
	LogDir               string
	PrintContainerOutput bool
	Images               map[string]string // client name -> image name
	ClientTypes          []string
}

// TestSuiteID identifies a test suite context.
type TestSuiteID uint32

func (tsID TestSuiteID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestID identifies a test case context.
type TestID uint32

func (tsID TestID) String() string {
	return strconv.Itoa(int(tsID))
}

// TestSuite is a single run of a simulator, a collection of testcases.
type TestSuite struct {
	ID          TestSuiteID          `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TestCases   map[TestID]*TestCase `json:"testCases"`
	// the log-file pertaining to the simulator. (may encompass more than just one TestSuite)
	SimulatorLog string `json:"simLog"`
}

// updateDB writes the simulation result to the log directory.
func (s *TestSuite) updateDB(logdir string) error {
	suiteData, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Randomize the name, but make it so that it's ordered by date - makes cleanups easier
	b := make([]byte, 16)
	rand.Read(b)
	suiteFileName := fmt.Sprintf("%v-%x.json", time.Now().Unix(), b)
	suiteFile := filepath.Join(logdir, suiteFileName)
	// Write it.
	return ioutil.WriteFile(suiteFile, suiteData, 0644)
}

// TestCase represents a single test case in a test suite.
type TestCase struct {
	ID            TestID                 `json:"id"`          // Test case reference number.
	Name          string                 `json:"name"`        // Test case short name.
	Description   string                 `json:"description"` // Test case long description in MD.
	Start         time.Time              `json:"start"`
	End           time.Time              `json:"end"`
	SummaryResult TestResult             `json:"summaryResult"` // The result of the whole test case.
	ClientResults map[string]*TestResult `json:"clientResults"` // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
	ClientInfo    map[string]*ClientInfo `json:"clientInfo"`    // Info about each client.
	pseudoInfo    map[string]*ClientInfo // registry of participating pseudos maintained for client maintenance and not part of the result database
}

// TestResult is the payload submitted to the EndTest endpoint.
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
}

// ClientInfo describes a client that participated in a test case.
type ClientInfo struct {
	ID              string    `json:"id"`
	IP              string    `json:"ip"`
	Name            string    `json:"name"`
	VersionInfo     string    `json:"versionInfo"` //URL to github repo + branch.
	InstantiatedAt  time.Time `json:"instantiatedAt"`
	LogFile         string    `json:"logFile"` //Absolute path to the logfile.
	WasInstantiated bool
}
