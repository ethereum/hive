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
	// StartTest starts a test case, which provides a context for clients and results, returning the test case identifier
	StartTest(name string, description string) (int, error)
	// EndTest ends a test case, cleaning up client instances created for the test and writing out results
	EndTest(test int, summaryResult TestResult, clientResults map[string]TestResult) error
	//Get a specific client's enode
	GetClientEnode(test int, node string) (*string, error)
	//Get all client types available to this simulator run
	//this depends on both the available client set
	//and the command line filters
	GetClientTypes() ([]string, error)
	//Start a new node (or other container) with the specified parameters
	//One parameter must be named CLIENT and should contain one of the
	//returned client types from GetClientTypes
	//The input is used as environment variables in the new container
	//Returns container id, ip and mac
	StartNewNode(test int, parameters map[string]string) (string, net.IP, string, error)
	//Start a new pseudo-client with the specified parameters
	//One parameter must be named CLIENT
	//The input is used as environment variables in the new container
	//Returns container id, ip and mac
	StartNewPseudo(test int, parameters map[string]string) (string, net.IP, string, error)

	//Signal that a node is no longer required
	KillNode(test int, node string) error
}

//Logger a general logger interface
type Logger interface {
	Log(args ...interface{})
	Logf(format string, args ...interface{})
}
