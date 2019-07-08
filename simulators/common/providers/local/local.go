package providers

import (
	"encoding/json"
	"errors"
	"math"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ethereum/hive/simulators/common"
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
// method.
//
type HostConfiguration struct {
	AvailableClients []ClientDescription `json:"clientDescription"`
}

// ClientDescription is metadata about the pre-supplied clients
type ClientDescription struct {
	IsPseudo      bool              `json:"isPseudo"`
	ClientType    string            `json:"clientType"`
	Parameters    map[string]string `json:"parameters,omitempty"`
	Enode         string            `json:"Enode,omitempty"`
	IP            net.IP            `json:"IP"`
	Mac           string            `json:"Mac"`
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
func GetInstance(config []byte) common.TestSuiteHost {

	once.Do(func() {

		var result HostConfiguration
		json.Unmarshal(config, &result)

		clientsByType, clientTypes, pseudosByType, pseudoTypes := mapClients()

		hostProxy = &host{
			configuration: &result,
			clientsByType: clientsByType,
			clientTypes:   clientTypes,
			pseudosByType: pseudosByType,
			pseudoTypes:   pseudoTypes,
		}

	})
	return hostProxy
}

func mapClients() (map[string][]int, []string, map[string][]int, []string) {

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
		pseudoTypes = append(clientTypes, k)
	}

	return clientsByType, clientTypes, pseudosByType, pseudoTypes
}

// EndTestSuite end
func (sim *host) EndTestSuite(testSuite common.TestSuiteID) error {
	//TODO -
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
		return nil, errors.New("no such node")
	}
	//return the enode
	return &sim.configuration.AvailableClients[nodeIndex].Enode, nil
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
		return 0, errors.New("no such test suite context")
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

//GetClientTypes Get all client types available to this simulator run
func (sim *host) GetClientTypes() (availableClients []string, err error) {
	return sim.clientTypes, nil
}

//StartNewNode attempts to acquire a new node matching the given parameters
//One parameter must be named CLIENT and should be a known type returned by GetClientTypes
//If there are multiple nodes, they will be selected round-robin
//Returns container id, ip, mac
func (sim *host) GetNode(test common.TestID, parameters map[string]string) (string, net.IP, string, error) {
	sim.nodeMutex.Lock()
	defer sim.nodeMutex.Unlock()

	client, ok := parameters["CLIENT"]
	if !ok {
		return "", nil, "", errors.New("unknown client")
	}

	availableClients, ok := sim.clientsByType[client]
	if !ok || len(availableClients) == 0 {
		return "", nil, "", errors.New("No available clients")
	}

	//select a node round-robin based on which is least used
	leastUsedCt := math.MaxUint32
	var leastUsed *ClientDescription
	var leastUsedIndex int
	for _, v := range availableClients {
		node := &sim.configuration.AvailableClients[v]
		if node.selectedCount < leastUsedCt {
			leastUsed = node
			leastUsedCt = node.selectedCount
			leastUsedIndex = v
		}
	}

	return strconv.Itoa(leastUsedIndex), leastUsed.IP, leastUsed.Mac, nil

}

// GetPseudo - just return a pseudo client , such as a relay
func (sim *host) GetPseudo(test common.TestID, parameters map[string]string) (string, net.IP, string, error) {

	client, ok := parameters["CLIENT"]
	if !ok {
		return "", nil, "", errors.New("Unknown pseudo")
	}

	availablePseudos, ok := sim.pseudosByType[client]
	if !ok || len(availablePseudos) == 0 {
		return "", nil, "", errors.New("No available pseudos")
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
