package common

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/google/uuid"
)

const IndexFileName string = "index.json"

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
}

// TestSuite is a single run of a simulator, a collection of testcases
type TestSuite struct {
	ID          TestSuiteID          `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TestCases   map[TestID]*TestCase `json:"testCases"`
}

// UpdateDB should be called on TestSuite completion to
// write out the test results and update indexes.
// NB Concurrent Hive processes should be able to safely update the index file
// as long as the file system supports POSIX-like atomic writes.
func (testSuite *TestSuite) UpdateDB(outputPath string) error {

	// write out the test suite as a json file of its own

	bytes, err := json.Marshal(*testSuite)
	if err != nil {
		return err
	}

	fileName := uuid.NewUUID()

	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(bytes)
	if err != nil {
		return err
	}

	// now update indexes:
	indexName := filepath.Join(outputPath, IndexFileName)

	return nil
}

// TestCase represents a single test case in a test suite
type TestCase struct {
	ID            TestID                     `json:"id"`          // Test case reference number.
	Name          string                     `json:"name"`        // Test case short name.
	Description   string                     `json:"description"` // Test case long description in MD.
	Start         time.Time                  `json:"start"`
	End           time.Time                  `json:"end"`
	SummaryResult TestResult                 `json:"summaryResult"` // The result of the whole test case.
	ClientResults map[string]*TestResult     `json:"clientResults"` // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
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
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	VersionInfo    string    `json:"versionInfo"` //URL to github repo + branch.
	InstantiatedAt time.Time `json:"instantiatedAt"`
	LogFile        string    `json:"logFile"` //Absolute path to the logfile.
}
