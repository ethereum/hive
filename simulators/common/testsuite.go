package common

import (
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
	ID          TestSuiteID
	Name        string
	Description string
	TestCases   map[TestID]*TestCase
}

// TestCase represents a single test case in a test suite
type TestCase struct {
	ID            TestID // Test case reference number.
	Name          string // Test case short name.
	Description   string // Test case long description in MD.
	Start         time.Time
	End           time.Time
	SummaryResult TestResult                 // The result of the whole test case.
	ClientResults map[string]*TestResult     // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
	ClientInfo    map[string]*TestClientInfo // Info about each client.
}

// TestResult describes the results of a test at the level of the overall test case and for each client involved in a test case
type TestResult struct {
	Pass    bool
	Details string
}

// TestClientInfo describes a client that participated in a test case
type TestClientInfo struct {
	ID             string
	Name           string
	VersionInfo    string //URL to github repo + branch.
	InstantiatedAt time.Time
	LogFile        string //Absolute path to the logfile.
}
