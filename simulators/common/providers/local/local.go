// Package local offers a Hive provider that allows users to run tests against
// a list of presupplied nodes and pseudo-nodes. This can be used to run
// p2p and rpc tests against running nodes without the need for Docker or other
// potential Hive provider dependencies. The responsibility of resetting node state
// between tests is placed on the user.
package local

import (
	"encoding/json"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/hive/simulators/common"
)

var (
	// ErrNoSuchNode no node with the requested id
	ErrNoSuchNode = errors.New("no such node")

	// ErrNoSuchTestSuite is that the test suite does not exist
	ErrNoSuchTestSuite = errors.New("no such test suite context")

	// ErrMissingClientType is that the operation was expecting a CLIENT parameter
	ErrMissingClientType = errors.New("missing client type")

	// ErrNoAvailableClients is the error that no pre-supplied clients of the requested type are available
	ErrNoAvailableClients = errors.New("no available clients")
)

// HostConfiguration is used to set up the local provider.
// It describes pre-supplied nodes. During tests and when nodes are requested,
// these pre-supplied nodes are selected
// according to the following rules:
// 1. Does the request general type (client/pseudo) match?
// 2. Does the type match? (Geth/Parity/Nethermind/Etc)
// 3. Does a configuration parameter exist in the supplied descriptor that is also
// in the request descriptor, and do they match?
// If multiple nodes are pre-supplied that fulfil requests, these are selected in round-robin
// method. Pseudos are not treated in this way.
//
type HostConfiguration struct {
	AvailableClients []ClientDescription `json:"availableClients"`
}

// ClientDescription is metadata about the pre-supplied clients
type ClientDescription struct {
	IsPseudo      bool              `json:"isPseudo"`
	ClientType    string            `json:"clientType"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	Enode         *string           `json:"Enode,omitempty"`
	IP            net.IP            `json:"ip"`
	Mac           *string           `json:"mac"`
	selectedCount int
}

type host struct {
	configuration     *HostConfiguration
	clientsByType     map[string][]int
	pseudosByType     map[string][]int
	clientTypes       []string
	pseudoTypes       []string
	nodeMutex         sync.Mutex
	runningTestSuites map[common.TestSuiteID]*common.TestSuite
	runningTestCases  map[common.TestID]*common.TestCase
	testCaseMutex     sync.Mutex
	testSuiteMutex    sync.Mutex
	testSuiteCounter  uint32
	testCaseCounter   uint32
}

var hostProxy *host
var once sync.Once

// GetInstance returns the instance of a local provider, which uses presupplied node instances and creates logs to a local destination,
// and provides a single opportunity to configure it during initialisation.
// The configuration is supplied as a byte representation, obtained from a file usually.
func GetInstance(config []byte, output io.Writer) (common.TestSuiteHost, error) {
	var err error
	once.Do(func() {
		err = generateInstance(config)
	})
	return hostProxy, err
}

//used in unit testing
func generateInstance(config []byte) error {
	var result HostConfiguration
	err := json.Unmarshal(config, &result)
	if err != nil {
		return err
	}

	hostProxy = &host{
		configuration:     &result,
		runningTestSuites: make(map[common.TestSuiteID]*common.TestSuite),
		runningTestCases:  make(map[common.TestID]*common.TestCase),
		clientsByType:     make(map[string][]int),
		pseudosByType:     make(map[string][]int),
	}

	mapClients()
	return nil
}

func mapClients() {

	clientsByType := make(map[string][]int)
	clientTypes := make([]string, 0)
	pseudosByType := make(map[string][]int)
	pseudoTypes := make([]string, 0)

	for i, v := range hostProxy.configuration.AvailableClients {
		if v.IsPseudo {

			pseudosByType[v.ClientType] = append(pseudosByType[v.ClientType], i)

		} else {

			clientsByType[v.ClientType] = append(clientsByType[v.ClientType], i)

		}
	}

	for k := range clientsByType {
		clientTypes = append(clientTypes, k)
	}

	for k := range pseudosByType {
		pseudoTypes = append(pseudoTypes, k)
	}

	hostProxy.clientsByType = clientsByType
	hostProxy.clientTypes = clientTypes
	hostProxy.pseudosByType = pseudosByType
	hostProxy.pseudoTypes = pseudoTypes
}

// EndTestSuite ends the test suite by writing the test suite results to the supplied
// stream and removing the test suite from the running list
func (sim *host) EndTestSuite(testSuite common.TestSuiteID) error {

	//Ending the test suite must write the data out to the supplied stream (io.Writer)
	//

	return nil

}

// GetClientEnode Get the client enode for the specified node id, which in this case is just the index
func (sim *host) GetClientEnode(test common.TestID, node string) (*string, error) {
	//local nodes are identified by their index
	nodeIndex, err := strconv.Atoi(node)
	if err != nil {
		return nil, err
	}
	// make sure it is within the bounds of the node list
	if nodeIndex < 0 || nodeIndex > len(sim.configuration.AvailableClients) {
		return nil, ErrNoSuchNode
	}
	//return the enode
	return sim.configuration.AvailableClients[nodeIndex].Enode, nil
}

// StartTestSuite starts a test suite and returns the context id
func (sim *host) StartTestSuite(name string, description string) common.TestSuiteID {

	sim.testSuiteMutex.Lock()
	defer sim.testSuiteMutex.Unlock()

	var newSuiteID = common.TestSuiteID(sim.testSuiteCounter)

	sim.runningTestSuites[newSuiteID] = &common.TestSuite{
		ID:          newSuiteID,
		Name:        name,
		Description: description,
		TestCases:   make(map[common.TestID]*common.TestCase),
	}

	sim.testSuiteCounter++

	return newSuiteID
}

//StartTest starts a new test case, returning the testcase id as a context identifier
func (sim *host) StartTest(testSuiteID common.TestSuiteID, name string, description string) (common.TestID, error) {

	sim.testCaseMutex.Lock()
	defer sim.testCaseMutex.Unlock()

	// check if the testsuite exists
	testSuite, ok := sim.runningTestSuites[testSuiteID]
	if !ok {
		return 0, ErrNoSuchTestSuite
	}

	// increment the testcasecounter
	sim.testCaseCounter++

	var newCaseID = common.TestID(sim.testCaseCounter)

	// create a new test case and add it to the test suite
	newTestCase := &common.TestCase{
		ID:          newCaseID,
		Name:        name,
		Description: description,
		Start:       time.Now(),
	}

	// add the test case to the test suite
	testSuite.TestCases[newCaseID] = newTestCase
	// and to the general map of id:testcases
	sim.runningTestCases[newCaseID] = newTestCase

	return newTestCase.ID, nil
}

// EndTest finishes the test case
func (sim *host) EndTest(testID common.TestID, summaryResult *common.TestResult, clientResults map[string]*common.TestResult) error {
	//	testCase := sim.runningTestCases[testID]
	// A local configuration might want to be informed
	// of the fact that the test case has ended in order to
	// reset state.
	// The main Hive Docker provider achieves this by killing
	// containers. The Local provider specifies pre-existing clients.
	// Signal that state should be reset by using the Kill message.

	// Check if the test case is running

	// Append the results to the test case

	// Signal that the nodes here should be reset

	//TODO:

	return nil
}

//GetClientTypes Get all client types available
func (sim *host) GetClientTypes() (availableClients []string, err error) {
	return append(sim.clientTypes, sim.pseudoTypes...), nil
}

// GetNode attempts to acquire a new node matching the given parameters
// One parameter must be named CLIENT and should be a known type returned by GetClientTypes
// If there are multiple nodes, they will be selected round-robin
// Returns node id, ip, mac
func (sim *host) GetNode(test common.TestID, parameters map[string]string) (string, net.IP, *string, error) {
	sim.nodeMutex.Lock()
	defer sim.nodeMutex.Unlock()

	client, ok := parameters["CLIENT"]
	if !ok {
		return "", nil, nil, ErrMissingClientType
	}

	availableClients, ok := sim.clientsByType[client]
	if !ok || len(availableClients) == 0 {
		return "", nil, nil, ErrNoAvailableClients
	}

	//select a node round-robin based on the parameters filter
	//and then the least used if multiple are returned
	leastUsedCt := math.MaxUint32
	var leastUsed *ClientDescription
	var leastUsedIndex int
	for _, v := range availableClients {
		node := &sim.configuration.AvailableClients[v]
		if matchFilter(node.Parameters, parameters) &&
			node.selectedCount < leastUsedCt {
			leastUsed = node
			leastUsedCt = node.selectedCount
			leastUsedIndex = v
		}
	}

	if leastUsed == nil {
		return "", nil, nil, ErrNoAvailableClients
	}

	return strconv.Itoa(leastUsedIndex), leastUsed.IP, leastUsed.Mac, nil
}

// matchFilter checks if the node description contains the specified key
// and if so rejects the node if the values do not match
func matchFilter(nodeDescription map[string]string, filter map[string]string) bool {
	for k, v := range filter {
		matched, ok := nodeDescription[k]
		if ok && matched != v {
			return false
		}
	}
	return true
}

// GetPseudo - just return a pseudo client , such as a relay
func (sim *host) GetPseudo(test common.TestID, parameters map[string]string) (string, net.IP, *string, error) {

	client, ok := parameters["CLIENT"]
	if !ok {
		return "", nil, nil, ErrMissingClientType
	}

	availablePseudos, ok := sim.pseudosByType[client]
	if !ok || len(availablePseudos) == 0 {
		return "", nil, nil, ErrNoAvailableClients
	}

	//The id is just the index in the list of all pseudos of the first pseudo of this type (we don't support
	//multiple pseudos)
	pseudoID := availablePseudos[0]
	pseudo := &sim.configuration.AvailableClients[pseudoID]

	return strconv.Itoa(pseudoID), pseudo.IP, pseudo.Mac, nil
}

// KillNode signals to the host that the node is no longer required
func (sim *host) KillNode(test common.TestID, node string) error {
	//Doing nothing for now.

	//todo
	return nil
}
