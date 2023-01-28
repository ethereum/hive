package libhive

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
	ID             TestSuiteID          `json:"id"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	ClientVersions map[string]string    `json:"clientVersions"`
	TestCases      map[TestID]*TestCase `json:"testCases"`
	// the log-file pertaining to the simulator. (may encompass more than just one TestSuite)
	SimulatorLog string `json:"simLog"`
}

// TestCase represents a single test case in a test suite.
type TestCase struct {
	Name          string                 `json:"name"`        // Test case short name.
	Description   string                 `json:"description"` // Test case long description in MD.
	Start         time.Time              `json:"start"`
	End           time.Time              `json:"end"`
	SummaryResult TestResult             `json:"summaryResult"` // The result of the whole test case.
	ClientInfo    map[string]*ClientInfo `json:"clientInfo"`    // Info about each client.
}

// TestResult is the payload submitted to the EndTest endpoint.
type TestResult struct {
	Pass    bool   `json:"pass"`
	Timeout bool   `json:"timeout,omitempty"`
	Details string `json:"details"`
}

// ClientInfo describes a client that participated in a test case.
type ClientInfo struct {
	ID             string    `json:"id"`
	IP             string    `json:"ip"`
	Name           string    `json:"name"`
	InstantiatedAt time.Time `json:"instantiatedAt"`
	LogFile        string    `json:"logFile"` //Absolute path to the logfile.

	wait func()
}

// HiveInstance contains information about hive itself.
type HiveInstance struct {
	SourceCommit string `json:"sourceCommit"`
	SourceDate   string `json:"sourceDate"`
	BuildDate    string `json:"buildDate"`
}

// ClientDefinition is served by the /clients API endpoint to list the available clients
type ClientDefinition struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Image   string         `json:"-"` // not exposed via API
	Meta    ClientMetadata `json:"meta"`
}

// ExecInfo is the result of running a script in a client container.
type ExecInfo struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}
