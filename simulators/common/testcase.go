package common

import "time"

// TestCase represents a single test case in a test suite
type TestCase interface {
	ID() int             // Test case reference number.
	Name() string        // Test case short name.
	Description() string // Test case long description in MD.
	Start() time.Time
	End() time.Time
	SummaryResult() TestResult             // The result of the whole test case.
	ClientResults() map[string]TestResult  // Client specific results, if this test case supports this concept. Not all test cases will identify a specific client as a test failure reason.
	ClientInfo() map[string]TestClientInfo // Info about each client.
	LogFile() string                       // Path to test case logfile
}

// TestResult describes the results of a test at the level of the overall test case and for each client involved in a test case
type TestResult interface {
	Pass() bool
	Details() string
}

// TestClientInfo describes a client that participated in a test case
type TestClientInfo interface {
	ID() string
	Name() string
	VersionInfo() string //URL to github repo + branch.
	InstantiatedAt() time.Time
	LogFile() string //Absolute path to the logfile.
}
