package libhive

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Docker label keys used by Hive
const (
	LabelHiveInstance    = "hive.instance"       // Unique Hive instance ID
	LabelHiveVersion     = "hive.version"        // Hive version/commit
	LabelHiveType        = "hive.type"           // container type: client|simulator|proxy
	LabelHiveTestSuite   = "hive.test.suite"     // test suite ID
	LabelHiveTestCase    = "hive.test.case"      // test case ID
	LabelHiveClientName  = "hive.client.name"    // client name (go-ethereum, etc)
	LabelHiveClientImage = "hive.client.image"   // Docker image name
	LabelHiveCreated     = "hive.created"        // RFC3339 timestamp
	LabelHiveSimulator   = "hive.simulator"      // simulator name
)

// Container types
const (
	ContainerTypeClient    = "client"
	ContainerTypeSimulator = "simulator"
	ContainerTypeProxy     = "proxy"
)

// GenerateHiveInstanceID creates a unique identifier for this Hive run
func GenerateHiveInstanceID() string {
	return fmt.Sprintf("hive-%d-%s", os.Getpid(), time.Now().Format("20060102-150405.000"))
}

// NewBaseLabels creates base labels for all containers
func NewBaseLabels(instanceID, hiveVersion string) map[string]string {
	return map[string]string{
		LabelHiveInstance: instanceID,
		LabelHiveVersion:  hiveVersion,
		LabelHiveCreated:  time.Now().Format(time.RFC3339),
	}
}

// SanitizeContainerNameComponent sanitizes a string for use in Docker container names
// Docker names must match [a-zA-Z0-9][a-zA-Z0-9_.-]*
func SanitizeContainerNameComponent(s string) string {
	if s == "" {
		return s
	}
	
	// Replace invalid characters with dashes
	sanitized := ""
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			sanitized += string(r)
		} else if i > 0 && (r == '_' || r == '.' || r == '-') {
			// Allow these characters after the first character
			sanitized += string(r)
		} else {
			// Replace invalid characters with dash
			sanitized += "-"
		}
	}
	
	// Ensure first character is alphanumeric
	if len(sanitized) > 0 && !((sanitized[0] >= 'a' && sanitized[0] <= 'z') || 
		(sanitized[0] >= 'A' && sanitized[0] <= 'Z') || 
		(sanitized[0] >= '0' && sanitized[0] <= '9')) {
		if len(sanitized) > 1 {
			sanitized = sanitized[1:]
		} else {
			sanitized = "container"
		}
	}
	
	return sanitized
}

// GenerateContainerName generates a Hive-prefixed container name
func GenerateContainerName(containerType, identifier string) string {
	timestamp := time.Now().Format("20060102-150405")
	sanitizedType := SanitizeContainerNameComponent(containerType)
	if identifier != "" {
		sanitizedIdentifier := SanitizeContainerNameComponent(identifier)
		return fmt.Sprintf("hive-%s-%s-%s", sanitizedType, sanitizedIdentifier, timestamp)
	}
	return fmt.Sprintf("hive-%s-%s", sanitizedType, timestamp)
}

// GenerateClientContainerName generates a name for client containers
func GenerateClientContainerName(clientName string, suiteID TestSuiteID, testID TestID) string {
	identifier := fmt.Sprintf("%s-s%s-t%s", clientName, suiteID.String(), testID.String())
	return GenerateContainerName("client", identifier)
}

// GenerateSimulatorContainerName generates a name for simulator containers
func GenerateSimulatorContainerName(simulatorName string) string {
	return GenerateContainerName("simulator", simulatorName)
}

// GenerateProxyContainerName generates a name for hiveproxy containers
func GenerateProxyContainerName() string {
	return GenerateContainerName("proxy", "")
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
	ID             TestSuiteID          `json:"id"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	ClientVersions map[string]string    `json:"clientVersions"`
	RunMetadata    *RunMetadata         `json:"runMetadata,omitempty"` // Enhanced run metadata
	TestCases      map[TestID]*TestCase `json:"testCases"`

	SimulatorLog   string `json:"simLog"`         // path to simulator log-file simulator. (may be shared with multiple suites)
	TestDetailsLog string `json:"testDetailsLog"` // the test details output file

	testDetailsFile *os.File
	testLogOffset   int64
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

// TestResult represents the result of a test case.
type TestResult struct {
	Pass    bool `json:"pass"`
	Timeout bool `json:"timeout,omitempty"`

	// The test log can be stored inline ("details"), or as offsets into the
	// suite's TestDetailsLog file ("log").
	Details    string          `json:"details,omitempty"`
	LogOffsets *TestLogOffsets `json:"log,omitempty"`
}

type TestLogOffsets struct {
	Begin int64 `json:"begin"`
	End   int64 `json:"end"`
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
	SourceCommit string      `json:"sourceCommit"`
	SourceDate   string      `json:"sourceDate"`
	BuildDate    string      `json:"buildDate"`
	HiveVersion  VersionInfo `json:"hiveVersion,omitempty"` // Enhanced hive version info
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
