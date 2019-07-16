package common

import (
	"fmt"
	"strconv"
	"time"
)

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

// TestSuite is a single run of a simulator, a collection of testcases
type TestSuite struct {
	ID          TestSuiteID          `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	TestCases   map[TestID]*TestCase `json:"testCases"`
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
func (t TestResult) AddDetail(detail string) {
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
