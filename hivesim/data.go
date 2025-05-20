package hivesim

import "slices"

// SuiteID identifies a test suite context.
type SuiteID uint32

// TestID identifies a test case context.
type TestID uint32

// TestResult describes the outcome of a test.
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
}

// TestStartInfo contains metadata about a test which is supplied to the hive API.
type TestStartInfo struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Location    string `json:"location"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// LogOffset tracks the start and end positions in a log file.
type LogOffset struct {
	Start int64 `json:"start"` // Byte offset where this section begins
	End   int64 `json:"end"`   // Byte offset where this section ends
}

// ClientLogInfo tracks log offsets for a specific client in a test.
type ClientLogInfo struct {
	ClientID  string    `json:"client_id"` // ID of the client container
	LogOffset LogOffset `json:"log_offset"` // Offset range in the log file
}

// ExecInfo is the result of running a command in a client container.
type ExecInfo struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exitCode"`
}

// ClientMetadata is part of the ClientDefinition and lists metadata
type ClientMetadata struct {
	Roles []string `yaml:"roles" json:"roles"`
}

// ClientDefinition is served by the /clients API endpoint to list the available clients
type ClientDefinition struct {
	Name    string         `json:"name"`
	Version string         `json:"version"`
	Meta    ClientMetadata `json:"meta"`
}

// HasRole reports whether the client has the given role.
func (m *ClientDefinition) HasRole(role string) bool {
	return slices.Contains(m.Meta.Roles, role)
}

// ClientMode defines whether a client is shared across tests or dedicated to a single test.
// Two modes are supported: DedicatedClient (default) and SharedClient.
type ClientMode int

const (
	// DedicatedClient is a client that is used for a single test (default behavior)
	DedicatedClient ClientMode = iota
	// SharedClient is a client that is shared across multiple tests in a suite
	SharedClient
)

// SharedClientInfo contains information about a shared client instance.
// This includes container identification and connectivity information.
type SharedClientInfo struct {
	ID        string // Container ID
	Type      string // Client type
	IP        string // Client IP address
	CreatedAt int64  // Timestamp when client was created
	LogFile   string // Path to client log file
}
