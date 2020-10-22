package common

import (
	"net"
)

// TestSuiteHost is implemented by the simulator API client.
type TestSuiteHost interface {
	// StartTestSuite registers a test suite. You need to call this at the beginning
	// of the simulation to get the suite ID.
	StartTestSuite(string, description, simlog string) (TestSuiteID, error)

	// StartTest creates a test case, which is the context for clients and results.
	StartTest(testSuiteRun TestSuiteID, name string, description string) (TestID, error)

	// EndTest ends a test case. This cleans up client instances created for the test
	// and writes out the results.
	EndTest(testSuiteRun TestSuiteID, test TestID, summaryResult *TestResult, clientResults map[string]*TestResult) error

	// EndTestSuite ends a test suite and triggers output of the whole suite results.
	EndTestSuite(testSuite TestSuiteID) error

	// GetClientTypes returns all client types available to this simulator run.
	// This depends on both the available client definitions and the command line filters.
	GetClientTypes() ([]string, error)

	// GetClientEnode returns the enode URL of a running client.
	GetClientEnode(testSuite TestSuiteID, test TestID, node string) (*string, error)

	// GetNode starts a client container.
	//
	// The "CLIENT" parameter selects the client implementation which is run.
	// This parameter is required and must be one of the client types returned by GetClientTypes.
	//
	// Other parameters configure the client container. The parameters and their values
	// are forwarded to the client container as environment variables.
	//
	// initFiles is a map of initialising files (eg: chain.rlp, blocks, genesis.json etc).
	//
	// Returns container ID, IP and MAC address.
	GetNode(testSuite TestSuiteID, test TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, *string, error)

	// GetPseudo starts a new pseudo-client with the specified parameters.
	// The "CLIENT" parameter selects which pseudoclient is started.
	//
	// The parameters are passed to the container as environment variables.
	// Returns container ID, IP and MAC address.
	GetPseudo(testSuite TestSuiteID, test TestID, parameters map[string]string) (string, net.IP, *string, error)

	// KillNode terminates the given client container. Note that calling this is usually
	// not required because all container will be terminated when calling EndTest.
	KillNode(testSuite TestSuiteID, test TestID, node string) error
}

// MacENREntry a type of ENR record for holding mac addresses
type MacENREntry string

// ENRKey the key for this type of ENR record
func (v MacENREntry) ENRKey() string { return "mac" }

// Logger is a general logger interface.
type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}
