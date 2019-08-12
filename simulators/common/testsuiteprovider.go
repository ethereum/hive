package common

import (
	"io/ioutil"
	"net"
)

//MacENREntry a type of ENR record for holding mac addresses
type MacENREntry string

//ENRKey the key for this type of ENR record
func (v MacENREntry) ENRKey() string { return "mac" }

//TestSuiteProviderInitialiser is the singleton getter initialising or returning the provider
type TestSuiteProviderInitialiser func(config []byte) (TestSuiteHost, error)

// TestSuiteHostProviders is the dictionary of test suit host providers
var testSuiteHostProviders = make(map[string]TestSuiteProviderInitialiser)

// RegisterProvider allows a test suite host provider to be supported
func RegisterProvider(key string, provider TestSuiteProviderInitialiser) {
	testSuiteHostProviders[key] = provider
}

//TestSuiteHost The test suite host the simulator communicates with to manage test cases and their resources
type TestSuiteHost interface {
	StartTestSuite(name string, description string) (TestSuiteID, error)
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
	//Returns container id, ip and mac
	GetNode(testSuite TestSuiteID, test TestID, parameters map[string]string) (string, net.IP, *string, error)
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

//InitProvider initialises and returns a testsuite provider singleton. Testsuite providers
//deliver the test suite and case maintenance services needed for simulations to run.
func InitProvider(providerName string, providerConfigFileName string) (TestSuiteHost, error) {

	providerIniter, ok := testSuiteHostProviders[providerName]
	if !ok {
		return nil, ErrNoSuchProviderType
	}

	configFileBytes, err := ioutil.ReadFile(providerConfigFileName)
	if err != nil {
		return nil, err
	}

	host, err := providerIniter(configFileBytes)
	if err != nil {
		return nil, err
	}
	return host, nil
}
