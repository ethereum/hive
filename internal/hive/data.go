// Package hive contains shared types for hive.
package hive

import (
	"strconv"
	"time"
)

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
	MAC             string    `json:"mac"` // TODO: remove this
	Name            string    `json:"name"`
	VersionInfo     string    `json:"versionInfo"` //URL to github repo + branch.
	InstantiatedAt  time.Time `json:"instantiatedAt"`
	LogFile         string    `json:"logFile"` //Absolute path to the logfile.
	WasInstantiated bool
}
