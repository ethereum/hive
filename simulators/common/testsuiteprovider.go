package common

import (
	"net"
)

//MacENREntry a type of ENR record for holding mac addresses
type MacENREntry string

//ENRKey the key for this type of ENR record
func (v MacENREntry) ENRKey() string { return "mac" }

//TestSuiteHost The test suite host the simulator communicates with to manage test cases and their resources
type TestSuiteHost interface {
	StartTestSuite(string, description, simlog string) (TestSuiteID, error)
	// StartTest starts a test case, which provides a context for clients and results, returning the test case identifier
	StartTest(testSuiteRun TestSuiteID, name string, description string) (TestID, error)
	// EndTest ends a test case, cleaning up client instances created for the test and writing out results
	EndTest(testSuiteRun TestSuiteID, test TestID, summaryResult *TestResult, clientResults map[string]*TestResult) error
	// EndTestSuite ends a test suite and triggers output of the whole suite results
	EndTestSuite(testSuite TestSuiteID) error
	//Get a specific client's enode
	GetClientEnode(testSuite TestSuiteID, test TestID, node string) (*string, error)
	//Get all client types available to this simulator run
	//this depends on both the available client set
	//and the command line filters
	GetClientTypes() ([]string, error)
	//GetNode gets a new (or pre-supplied) node
	//One parameter must be named CLIENT and should contain one of the
	//returned client types from GetClientTypes
	//The input is used as environment variables in the new container
	//initFiles is a dictionary of initialising files (eg: chain.rlp, blocks, genesis etc).
	//Returns container id, ip and mac
	GetNode(testSuite TestSuiteID, test TestID, parameters map[string]string, initFiles map[string]string) (string, net.IP, *string, error)
	// GetContainerNetworkIP gets the given container's IP address on the given network.
	GetContainerNetworkIP(testSuite TestSuiteID, networkID, containerID string) (string, error)
	// ConnectContainer connects the given container to the given network.
	ConnectContainer(testSuite TestSuiteID, networkID, containerID string) error
	// DisconnectContainer disconnects the given container from the given network.
	DisconnectContainer(testSuite TestSuiteID, networkID, containerID string) error
	// CreateNetwork creates a network by the given name, returning the network ID.
	CreateNetwork(testSuite TestSuiteID, networkName string) (string, error)
	// RemoveNetwork removes a network by the given networkID.
	RemoveNetwork(testSuite TestSuiteID, networkID string) error
	//GetPseudo gets a new (or pre-supplied) pseudo-client with the specified parameters
	//One parameter must be named CLIENT
	//The input is used as environment variables in the new container
	//Returns container id, ip and mac
	GetPseudo(testSuite TestSuiteID, test TestID, parameters map[string]string) (string, net.IP, *string, error)
	//Signal that a node is no longer required
	KillNode(testSuite TestSuiteID, test TestID, node string) error
}

//Logger a general logger interface
type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}
