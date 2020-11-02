package common

import (
	"fmt"
	"sync"
	"time"

	docker "github.com/fsouza/go-dockerclient"
)

// TestManager offers providers a common implementation for
// managing tests. It is a partial implementation of
// the TestSuiteHost interface
type TestManager struct {
	OutputPath       string
	KillNodeCallback func(testSuite TestSuiteID, test TestID, node string) error

	dockerClient        *docker.Client
	simulationContainer *docker.Container
	networks            map[string]string // map of all networks started by the test, key is network ID and value is network name.
	networkMutex        sync.RWMutex

	testLimiter       int
	runningTestSuites map[TestSuiteID]*TestSuite
	runningTestCases  map[TestID]*TestCase
	testCaseMutex     sync.RWMutex
	testSuiteMutex    sync.RWMutex
	nodeMutex         sync.Mutex
	pseudoMutex       sync.Mutex
	testSuiteCounter  uint32
	testCaseCounter   uint32
}

// NewTestManager is a constructor returning a TestManager
func NewTestManager(outputPath string, testLimiter int, killNodeCallback func(testSuite TestSuiteID, test TestID, node string) error, client *docker.Client) *TestManager {
	return &TestManager{
		OutputPath:        outputPath,
		testLimiter:       testLimiter,
		KillNodeCallback:  killNodeCallback,
		runningTestSuites: make(map[TestSuiteID]*TestSuite),
		runningTestCases:  make(map[TestID]*TestCase),
		dockerClient:      client,
		networks:          make(map[string]string),
	}
}

// IsTestSuiteRunning checks if the test suite is still running and returns it if so
func (manager *TestManager) IsTestSuiteRunning(testSuite TestSuiteID) (*TestSuite, bool) {
	manager.testSuiteMutex.RLock()
	defer manager.testSuiteMutex.RUnlock()
	suite, ok := manager.runningTestSuites[testSuite]
	return suite, ok
}

// IsTestRunning checks if the testis still running and returns it if so
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
				//kill any running tests and ensure that the host is
				//notified to clean up any resources (eg docker containers)
				err := manager.EndTest(suiteID, testID, terminationSummary, nil)
				if err != nil {
					return err
				}
			}
		}
		//ensure the db is updated with results
		manager.EndTestSuite(suiteID)
	}
	return nil
}

// GetNodeInfo gets some info on a client or pseudo belonging to some test
func (manager *TestManager) GetNodeInfo(testSuite TestSuiteID, test TestID, nodeID string) (*TestClientInfo, error) {
	manager.nodeMutex.Lock()
	defer manager.nodeMutex.Unlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return nil, ErrNoSuchTestSuite
	}
	testCase, ok := manager.IsTestRunning(test)
	if !ok {
		return nil, ErrNoSuchTestCase
	}
	nodeInfo, ok := testCase.ClientInfo[nodeID]
	if !ok {
		nodeInfo, ok = testCase.pseudoInfo[nodeID]
		if !ok {
			return nil, ErrNoSuchNode
		}
	}
	return nodeInfo, nil
}

// AddSimContainer adds the given simulation container to the test manager
// for later access.
func (manager *TestManager) AddSimContainer(container *docker.Container) {
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()
	manager.simulationContainer = container
}

// GetNetworkID returns the network ID of a given network if it exists.
func (manager *TestManager) GetNetworkID(testSuite TestSuiteID, networkName string) (string, error) {
	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return "", ErrNoSuchTestSuite
	}
	// range through networks, return ID if names match
	existing, err := manager.dockerClient.ListNetworks()
	if err != nil {
		return "", err
	}
	for _, exists := range existing {
		if exists.Name == networkName {
			return exists.ID, nil
		}
	}
	return "", fmt.Errorf("network not found")
}

// CreateNetwork creates a docker network with the given network name, returning
// the network ID upon success.
func (manager *TestManager) CreateNetwork(testSuite TestSuiteID, networkName string) (string, error) {
	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return "", ErrNoSuchTestSuite
	}
	// list networks to make sure not to duplicate
	existing, err := manager.dockerClient.ListNetworks()
	if err != nil {
		return "", err
	}
	// check for existing networks with same name, and if exists, remove
	for _, exists := range existing {
		if exists.Name == networkName {
			if err := manager.dockerClient.RemoveNetwork(exists.ID); err != nil {
				return "", err
			}
		}
	}
	// create network
	network, err := manager.dockerClient.CreateNetwork(docker.CreateNetworkOptions{
		Name:           networkName,
		Driver:         "bridge",
		CheckDuplicate: true,
		Attachable:     true,
	})
	if err != nil {
		return "", err
	}
	// add network to network map
	manager.networkMutex.Lock()
	manager.networks[network.ID] = network.Name
	manager.networkMutex.Unlock()
	return network.ID, err
}

// CreateNetwork creates a docker network with the given network name, returning
// the network ID upon success.
func (manager *TestManager) RemoveNetwork(networkID string) error {
	if err := manager.dockerClient.RemoveNetwork(networkID); err != nil {
		return err
	}
	manager.networkMutex.Lock()
	delete(manager.networks, networkID)
	manager.networkMutex.Unlock()
	return nil
}

// PruneNetworks prunes all unused docker network that were started by the test suite.
func (manager *TestManager) PruneNetworks() []error {
	var errs []error
	for id, _ := range manager.networks {
		if err := manager.RemoveNetwork(id); err != nil {
			errs = append(errs, err)
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
	// if the containerID is "simulation", use simulation container ID
	containerID, err := manager.isSimulation(containerID)
	if err != nil {
		return "", err
	}

	ipAddr, err := getContainerIP(manager.dockerClient, networkID, containerID)
	if err != nil {
		return "", err
	}

	return ipAddr, nil
}

func getContainerIP(dockerClient *docker.Client, networkID, container string) (string, error) {
	details, err := dockerClient.InspectContainerWithOptions(docker.InspectContainerOptions{
		ID: container,
	})
	if err != nil {
		return "", err
	}
	// range over all networks to which the container is connected
	// and get network-specific IPs
	for _, network := range details.NetworkSettings.Networks {
		if network.NetworkID == networkID {
			return network.IPAddress, nil
		}
	}
	return "", fmt.Errorf("network not found")
}

// ConnectContainer connects the given container to the given network.
func (manager *TestManager) ConnectContainer(testSuite TestSuiteID, networkID, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	// if the containerID is "simulation", use simulation container ID
	containerID, err := manager.isSimulation(containerID)
	if err != nil {
		return err
	}
	return manager.dockerClient.ConnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

// DisconnectContainer disconnects the given container from the given network.
func (manager *TestManager) DisconnectContainer(testSuite TestSuiteID, networkID, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	// if the containerID is "simulation", use simulation container ID
	containerID, err := manager.isSimulation(containerID)
	if err != nil {
		return err
	}
	return manager.dockerClient.DisconnectNetwork(networkID, docker.NetworkConnectionOptions{
		Container: containerID,
	})
}

func (manager *TestManager) isSimulation(nodeID string) (string, error) {
	if nodeID != "simulation" {
		return nodeID, nil
	}
	if manager.simulationContainer == nil {
		return "", fmt.Errorf("simulation container not found")
	}
	return manager.simulationContainer.ID, nil
}

// EndTestSuite ends the test suite by writing the test suite results to the supplied
// stream and removing the test suite from the running list
func (manager *TestManager) EndTestSuite(testSuite TestSuiteID) error {
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	// check the test suite exists
	suite, ok := manager.runningTestSuites[testSuite]
	if !ok {
		return ErrNoSuchTestSuite
	}
	// check the suite has no running test cases
	for k := range suite.TestCases {
		_, ok := manager.runningTestCases[k]
		if ok {
			return ErrTestSuiteRunning
		}
	}
	// update the db
	err := suite.UpdateDB(manager.OutputPath)
	if err != nil {
		return err
	}
	//remove the test suite
	delete(manager.runningTestSuites, testSuite)

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
		ClientInfo:  make(map[string]*TestClientInfo),
		pseudoInfo:  make(map[string]*TestClientInfo),
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

	for k, v := range testCase.ClientInfo {
		if v.WasInstantiated {
			manager.KillNodeCallback(testSuiteRun, testID, k)
		}
	}
	for k := range testCase.pseudoInfo {
		manager.KillNodeCallback(testSuiteRun, testID, k)
	}

	// Delete from running, if it's still there
	manager.testCaseMutex.Lock()
	if tc, ok := manager.runningTestCases[testID]; ok {
		delete(manager.runningTestCases, tc.ID)
	}
	manager.testCaseMutex.Unlock()

	return nil
}

// RegisterNode is used by test suite hosts to register the creation of a node in the context of a test
func (manager *TestManager) RegisterNode(testID TestID, nodeID string, nodeInfo *TestClientInfo) error {
	manager.nodeMutex.Lock()
	defer manager.nodeMutex.Unlock()

	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}

	testCase.ClientInfo[nodeID] = nodeInfo
	return nil
}

// RegisterPseudo is used by test suite hosts to register the creation of a node in the context of a test
func (manager *TestManager) RegisterPseudo(testID TestID, nodeID string, nodeInfo *TestClientInfo) error {
	manager.pseudoMutex.Lock()
	defer manager.pseudoMutex.Unlock()

	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}

	testCase.pseudoInfo[nodeID] = nodeInfo
	return nil
}
