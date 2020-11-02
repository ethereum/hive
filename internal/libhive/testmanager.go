package libhive

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

var (
	ErrNoSuchNode               = errors.New("no such node")
	ErrNoSuchTestSuite          = errors.New("no such test suite")
	ErrNoSuchTestCase           = errors.New("no such test case")
	ErrMissingClientType        = errors.New("missing client type")
	ErrNoAvailableClients       = errors.New("no available clients")
	ErrTestSuiteRunning         = errors.New("test suite still has running tests")
	ErrMissingOutputDestination = errors.New("test suite requires an output")
	ErrNoSummaryResult          = errors.New("test case must be ended with a summary result")
	ErrDBUpdateFailed           = errors.New("could not update results set")
	ErrTestSuiteLimited         = errors.New("testsuite test count is limited")
)

// TestManager collects test results during a simulation run.
type TestManager struct {
	config      SimEnv
	backend     Backend
	testLimiter int

	simContainerID string

	// map of all networks started by the test, key is network ID and value is network name.
	networks     map[string]string
	networkMutex sync.RWMutex

	testCaseMutex     sync.RWMutex
	testSuiteMutex    sync.RWMutex
	runningTestSuites map[TestSuiteID]*TestSuite
	runningTestCases  map[TestID]*TestCase
	testSuiteCounter  uint32
	testCaseCounter   uint32
	results           map[TestSuiteID]*TestSuite
}

func NewTestManager(config SimEnv, b Backend, testLimiter int) *TestManager {
	return &TestManager{
		config:            config,
		backend:           b,
		testLimiter:       testLimiter,
		runningTestSuites: make(map[TestSuiteID]*TestSuite),
		runningTestCases:  make(map[TestID]*TestCase),
		results:           make(map[TestSuiteID]*TestSuite),
		networks:          make(map[string]string),
	}
}

// SetSimContainerID sets the container ID of the simulation container. This must be called
// after creating the simulation container.
func (manager *TestManager) SetSimContainerID(id string) {
	manager.simContainerID = id
}

// Results returns the results for all suites that have already ended.
func (manager *TestManager) Results() map[TestSuiteID]*TestSuite {
	manager.testSuiteMutex.RLock()
	defer manager.testSuiteMutex.RUnlock()

	// Copy results.
	r := make(map[TestSuiteID]*TestSuite)
	for id, suite := range manager.results {
		r[id] = suite
	}
	return r
}

// API returns the simulation API handler.
func (manager *TestManager) API() http.Handler {
	return newSimulationAPI(manager.backend, manager.config, manager)
}

// IsTestSuiteRunning checks if the test suite is still running and returns it if so
func (manager *TestManager) IsTestSuiteRunning(testSuite TestSuiteID) (*TestSuite, bool) {
	manager.testSuiteMutex.RLock()
	defer manager.testSuiteMutex.RUnlock()
	suite, ok := manager.runningTestSuites[testSuite]
	return suite, ok
}

// IsTestRunning checks if the test is still running and returns it if so.
func (manager *TestManager) IsTestRunning(test TestID) (*TestCase, bool) {
	manager.testCaseMutex.RLock()
	defer manager.testCaseMutex.RUnlock()
	testCase, ok := manager.runningTestCases[test]
	return testCase, ok
}

// Terminate forces the termination of any running tests with
// an error message. This can be called as a cleanup method.
// If there are no running tests, there is no effect.
func (manager *TestManager) Terminate() error {
	terminationSummary := &TestResult{
		Pass:    false,
		Details: "Test was terminated by host",
	}
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	for suiteID, suite := range manager.runningTestSuites {
		for testID := range suite.TestCases {
			if _, running := manager.IsTestRunning(testID); running {
				// end any running tests and ensure that the host is notified to clean up
				// any resources (e.g. docker containers).
				err := manager.EndTest(suiteID, testID, terminationSummary, nil)
				if err != nil {
					return err
				}
			}
		}
		// ensure the db is updated with results
		manager.EndTestSuite(suiteID)
	}

	// Remove left-over docker networks.
	manager.PruneNetworks()

	return nil
}

// GetNodeInfo gets some info on a client or pseudo belonging to some test
func (manager *TestManager) GetNodeInfo(testSuite TestSuiteID, test TestID, nodeID string) (*ClientInfo, error) {
	manager.testCaseMutex.RLock()
	defer manager.testCaseMutex.RUnlock()

	testCase, ok := manager.runningTestCases[test]
	if !ok {
		return nil, ErrNoSuchTestCase
	}
	nodeInfo, ok := testCase.ClientInfo[nodeID]
	if !ok {
		nodeInfo, ok = testCase.PseudoInfo[nodeID]
		if !ok {
			return nil, ErrNoSuchNode
		}
	}
	return nodeInfo, nil
}

// CreateNetwork creates a docker network with the given network name, returning
// the network ID upon success.
func (manager *TestManager) CreateNetwork(testSuite TestSuiteID, networkName string) (string, error) {
	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return "", ErrNoSuchTestSuite
	}

	// add network to network map
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()

	netID, err := manager.backend.CreateNetwork(networkName)
	if err != nil {
		manager.networks[netID] = networkName
	}
	return netID, err
}

// CreateNetwork creates a docker network with the given network name, returning
// the network ID upon success.
func (manager *TestManager) RemoveNetwork(networkID string) error {
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()

	if err := manager.backend.RemoveNetwork(networkID); err != nil {
		return err
	}
	delete(manager.networks, networkID)
	return nil
}

// PruneNetworks removes all created networks.
func (manager *TestManager) PruneNetworks() []error {
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()

	var errs []error
	for id, name := range manager.networks {
		log15.Info("removing docker network", "id", id, "name", name)
		if err := manager.backend.RemoveNetwork(id); err != nil {
			errs = append(errs, err)
		} else {
			delete(manager.networks, id)
		}
	}
	return errs
}

// ContainerIP gets the IP address of the given container on the given network.
func (manager *TestManager) ContainerIP(testSuite TestSuiteID, networkID, containerID string) (string, error) {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return "", ErrNoSuchTestSuite
	}

	if containerID == "simulation" {
		containerID = manager.simContainerID
	}
	ipAddr, err := manager.backend.ContainerIP(containerID, networkID)
	if err != nil {
		return "", err
	}
	return ipAddr.String(), nil
}

// ConnectContainer connects the given container to the given network.
func (manager *TestManager) ConnectContainer(testSuite TestSuiteID, networkID, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	if containerID == "simulation" {
		containerID = manager.simContainerID
	}
	return manager.backend.ConnectContainer(containerID, networkID)
}

// DisconnectContainer disconnects the given container from the given network.
func (manager *TestManager) DisconnectContainer(testSuite TestSuiteID, networkID, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	if containerID == "simulation" {
		containerID = manager.simContainerID
	}
	return manager.backend.DisconnectContainer(containerID, networkID)
}

// EndTestSuite ends the test suite by writing the test suite results to the supplied
// stream and removing the test suite from the running list
func (manager *TestManager) EndTestSuite(testSuite TestSuiteID) error {
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	suite, ok := manager.runningTestSuites[testSuite]
	if !ok {
		return ErrNoSuchTestSuite
	}
	// Check the suite has no running test cases.
	for k := range suite.TestCases {
		_, ok := manager.runningTestCases[k]
		if ok {
			return ErrTestSuiteRunning
		}
	}
	// Write the result.
	if manager.config.LogDir != "" {
		err := suite.updateDB(manager.config.LogDir)
		if err != nil {
			return err
		}
	}
	// Move the suite to results.
	delete(manager.runningTestSuites, testSuite)
	manager.results[testSuite] = suite
	return nil
}

// StartTestSuite starts a test suite and returns the context id
func (manager *TestManager) StartTestSuite(name string, description string, simlog string) (TestSuiteID, error) {
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	var newSuiteID = TestSuiteID(manager.testSuiteCounter)
	manager.runningTestSuites[newSuiteID] = &TestSuite{
		ID:           newSuiteID,
		Name:         name,
		Description:  description,
		TestCases:    make(map[TestID]*TestCase),
		SimulatorLog: simlog,
	}
	manager.testSuiteCounter++
	return newSuiteID, nil
}

//StartTest starts a new test case, returning the testcase id as a context identifier
func (manager *TestManager) StartTest(testSuiteID TestSuiteID, name string, description string) (TestID, error) {

	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()
	// check if the testsuite exists
	testSuite, ok := manager.runningTestSuites[testSuiteID]
	if !ok {
		return 0, ErrNoSuchTestSuite
	}
	// check for a limiter
	if manager.testLimiter >= 0 && len(testSuite.TestCases) >= manager.testLimiter {
		return 0, ErrTestSuiteLimited
	}
	// increment the testcasecounter
	manager.testCaseCounter++
	var newCaseID = TestID(manager.testCaseCounter)
	// create a new test case and add it to the test suite
	newTestCase := &TestCase{
		ID:          newCaseID,
		Name:        name,
		Description: description,
		Start:       time.Now(),
	}
	// add the test case to the test suite
	testSuite.TestCases[newCaseID] = newTestCase
	// and to the general map of id:testcases
	manager.runningTestCases[newCaseID] = newTestCase

	return newTestCase.ID, nil
}

// EndTest finishes the test case
func (manager *TestManager) EndTest(testSuiteRun TestSuiteID, testID TestID, summaryResult *TestResult, clientResults map[string]*TestResult) error {
	manager.testCaseMutex.Lock()

	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		manager.testCaseMutex.Unlock()
		return ErrNoSuchTestCase
	}
	// Make sure there is at least a result summary
	if summaryResult == nil {
		manager.testCaseMutex.Unlock()
		return ErrNoSummaryResult
	}

	// Add the results to the test case
	testCase.End = time.Now()
	testCase.SummaryResult = *summaryResult
	testCase.ClientResults = clientResults
	manager.testCaseMutex.Unlock()

	// Stop running clients.
	for _, v := range testCase.ClientInfo {
		if v.WasInstantiated {
			manager.backend.StopContainer(v.ID)
		}
	}
	for _, v := range testCase.PseudoInfo {
		if v.WasInstantiated {
			manager.backend.StopContainer(v.ID)
		}
	}

	// Delete from running, if it's still there.
	manager.testCaseMutex.Lock()
	delete(manager.runningTestCases, testID)
	manager.testCaseMutex.Unlock()
	return nil
}

// RegisterNode is used by test suite hosts to register the creation of a node in the context of a test
func (manager *TestManager) RegisterNode(testID TestID, nodeID string, nodeInfo *ClientInfo) error {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}
	if testCase.ClientInfo == nil {
		testCase.ClientInfo = make(map[string]*ClientInfo)
	}
	testCase.ClientInfo[nodeID] = nodeInfo
	return nil
}

// RegisterPseudo is used by test suite hosts to register the creation of a node in the context of a test
func (manager *TestManager) RegisterPseudo(testID TestID, nodeID string, nodeInfo *ClientInfo) error {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}
	if testCase.PseudoInfo == nil {
		testCase.PseudoInfo = make(map[string]*ClientInfo)
	}
	testCase.PseudoInfo[nodeID] = nodeInfo
	return nil
}
