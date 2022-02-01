package hivesim

// SuiteID identifies a test suite context.
type SuiteID uint32

// TestID identifies a test case context.
type TestID uint32

// TestResult describes the outcome of a test.
type TestResult struct {
	Pass    bool   `json:"pass"`
	Details string `json:"details"`
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
	for _, m := range m.Meta.Roles {
		if m == role {
			return true
		}
	}
	return false
}
