package libhive

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	ErrNoSuchNode               = errors.New("no such node")
	ErrSharedClientNotRunning   = errors.New("shared client not running")
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

// SimEnv contains the simulation parameters.
type SimEnv struct {
	LogDir string

	// Parameters of simulation.
	SimLogLevel    int
	SimParallelism int
	SimRandomSeed  int
	SimTestPattern string
	SimBuildArgs   []string

	// This is the time limit for the simulation run.
	// There is no default limit.
	SimDurationLimit time.Duration

	// These are the clients which are made available to the simulator.
	// If unset (i.e. nil), all built clients are used.
	ClientList []ClientDesignator

	// This configures the amount of time the simulation waits
	// for the client to open port 8545 after launching the container.
	ClientStartTimeout time.Duration
}

// SimResult summarizes the results of a simulation run.
type SimResult struct {
	Suites       int
	SuitesFailed int
	Tests        int
	TestsFailed  int
}

// HiveInfo contains information about the hive instance running the simulation.
type HiveInfo struct {
	Command        []string           `json:"command"`
	ClientFile     []ClientDesignator `json:"clientFile"`
	ClientFilePath string             `json:"clientFilePath,omitempty"`
	Commit         string             `json:"commit"`
	Date           string             `json:"date"`
}

// TestManager collects test results during a simulation run.
type TestManager struct {
	config     SimEnv
	backend    ContainerBackend
	clientDefs []*ClientDefinition
	hiveInfo   HiveInfo

	simContainerID string
	simLogFile     string

	// Hive instance information for labeling
	hiveInstanceID string
	hiveVersion    string

	// all networks started by a specific test suite, where key
	// is network name and value is network ID
	networks     map[TestSuiteID]map[string]string
	networkMutex sync.RWMutex

	testCaseMutex     sync.RWMutex
	testSuiteMutex    sync.RWMutex
	runningTestSuites map[TestSuiteID]*TestSuite
	runningTestCases  map[TestID]*TestCase
	testSuiteCounter  uint32
	testCaseCounter   uint32
	results           map[TestSuiteID]*TestSuite
}

// filterClientDesignators removes sensitive build arguments from ClientDesignator slice
func filterClientDesignators(clients []ClientDesignator) []ClientDesignator {
	filtered := make([]ClientDesignator, len(clients))
	for i, client := range clients {
		filteredClient := ClientDesignator{
			Client:        client.Client,
			Nametag:       client.Nametag,
			DockerfileExt: client.DockerfileExt,
			BuildArgs:     make(map[string]string),
		}

		// Filter build args
		for key, value := range client.BuildArgs {
			if !excludedBuildArgs[key] {
				filteredClient.BuildArgs[key] = value
			}
		}

		filtered[i] = filteredClient
	}
	return filtered
}

func NewTestManager(config SimEnv, b ContainerBackend, clients []*ClientDefinition, hiveInfo HiveInfo) *TestManager {
	if hiveInfo.Commit == "" && hiveInfo.Date == "" {
		hiveInfo.Commit, hiveInfo.Date = hiveVersion()
	}

	// Filter sensitive build args from HiveInfo.ClientFile
	hiveInfo.ClientFile = filterClientDesignators(hiveInfo.ClientFile)

	return &TestManager{
		clientDefs:        clients,
		config:            config,
		backend:           b,
		hiveInfo:          hiveInfo,
		hiveInstanceID:    GenerateHiveInstanceID(),
		hiveVersion:       hiveInfo.Commit,
		runningTestSuites: make(map[TestSuiteID]*TestSuite),
		runningTestCases:  make(map[TestID]*TestCase),
		results:           make(map[TestSuiteID]*TestSuite),
		networks:          make(map[TestSuiteID]map[string]string),
	}
}

// SetSimContainerInfo makes the manager aware of the simulation container.
// This must be called after creating the simulation container, but before starting it.
func (manager *TestManager) SetSimContainerInfo(id, logFile string) {
	manager.simContainerID = id
	manager.simLogFile = logFile
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
	return newSimulationAPI(manager.backend, manager.config, manager, manager.hiveInfo)
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
		Timeout: true,
		Details: "Test was terminated by host",
	}
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	for suiteID, suite := range manager.runningTestSuites {
		for testID := range suite.TestCases {
			if _, running := manager.IsTestRunning(testID); running {
				// end any running tests and ensure that the host is notified to clean up
				// any resources (e.g. docker containers).
				err := manager.EndTest(suiteID, testID, terminationSummary)
				if err != nil {
					return err
				}
			}
		}
		// ensure the db is updated with results
		manager.doEndSuite(suiteID)
	}

	return nil
}

// GetNodeInfo gets some info on a client belonging to some test
func (manager *TestManager) GetNodeInfo(testSuite TestSuiteID, test *TestID, nodeID string) (*ClientInfo, error) {
	if test != nil {
		manager.testCaseMutex.RLock()
		defer manager.testCaseMutex.RUnlock()

		testCase, ok := manager.runningTestCases[*test]
		if !ok {
			return nil, ErrNoSuchTestCase
		}
		nodeInfo, ok := testCase.ClientInfo[nodeID]
		if !ok {
			return nil, ErrNoSuchNode
		}
		return nodeInfo, nil
	} else {
		manager.testSuiteMutex.RLock()
		defer manager.testSuiteMutex.RUnlock()

		testSuite, ok := manager.runningTestSuites[testSuite]
		if !ok {
			return nil, ErrNoSuchTestSuite
		}

		if testSuite.ClientInfo == nil {
			return nil, ErrNoSuchNode
		}
		client, ok := testSuite.ClientInfo[nodeID]
		if !ok {
			return nil, ErrNoSuchNode
		}
		return client, nil
	}
}

// CreateNetwork creates a docker network with the given network name.
func (manager *TestManager) CreateNetwork(testSuite TestSuiteID, name string) error {
	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}

	// add network to network map
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()

	id, err := manager.backend.CreateNetwork(getUniqueName(testSuite, name))
	if err != nil {
		return err
	}
	if _, exists := manager.networks[testSuite]; !exists {
		// initialize network map for individual test suite
		manager.networks[testSuite] = make(map[string]string)
	}
	manager.networks[testSuite][name] = id
	return nil
}

// getUniqueName returns a unique network name to prevent network collisions
func getUniqueName(testSuite TestSuiteID, name string) string {
	return fmt.Sprintf("hive_%d_%d_%s", os.Getpid(), testSuite, name)
}

// RemoveNetwork removes a docker network by the given network name.
func (manager *TestManager) RemoveNetwork(testSuite TestSuiteID, network string) error {
	manager.networkMutex.Lock()
	defer manager.networkMutex.Unlock()

	id, exists := manager.networks[testSuite][network]
	if !exists {
		return ErrNetworkNotFound
	}

	if err := manager.backend.RemoveNetwork(id); err != nil {
		return err
	}
	delete(manager.networks[testSuite], network)
	return nil
}

// PruneNetworks removes all networks created by the given test suite.
func (manager *TestManager) PruneNetworks(testSuite TestSuiteID) []error {
	var errs []error
	for name := range manager.networks[testSuite] {
		slog.Info("removing docker network", "name", name)
		if err := manager.RemoveNetwork(testSuite, name); err != nil {
			errs = append(errs, err)
		}
	}
	// delete the test suite from the network map as all its networks have been torn down
	manager.networkMutex.Lock()
	delete(manager.networks, testSuite)
	manager.networkMutex.Unlock()
	return errs
}

// ContainerIP gets the IP address of the given container on the given network.
func (manager *TestManager) ContainerIP(testSuite TestSuiteID, networkName, containerID string) (string, error) {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return "", ErrNoSuchTestSuite
	}

	if containerID == "simulation" {
		containerID = manager.simContainerID
	}

	var networkID string
	// networkID "bridge" is special.
	if networkName == "bridge" {
		var err error
		networkID, err = manager.backend.NetworkNameToID(networkName)
		if err != nil {
			return "", err
		}
	} else {
		var exists bool
		networkID, exists = manager.networks[testSuite][networkName]
		if !exists {
			return "", ErrNetworkNotFound
		}
	}

	ipAddr, err := manager.backend.ContainerIP(containerID, networkID)
	if err != nil {
		return "", err
	}
	return ipAddr.String(), nil
}

// ConnectContainer connects the given container to the given network.
func (manager *TestManager) ConnectContainer(testSuite TestSuiteID, networkName, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	if containerID == "simulation" {
		containerID = manager.simContainerID
	}

	networkID, exists := manager.networks[testSuite][networkName]
	if !exists {
		return ErrNetworkNotFound
	}
	return manager.backend.ConnectContainer(containerID, networkID)
}

// NetworkExists reports whether a network exists in the current test context.
func (manager *TestManager) NetworkExists(testSuite TestSuiteID, networkName string) bool {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, exists := manager.networks[testSuite][networkName]
	return exists
}

// DisconnectContainer disconnects the given container from the given network.
func (manager *TestManager) DisconnectContainer(testSuite TestSuiteID, networkName, containerID string) error {
	manager.networkMutex.RLock()
	defer manager.networkMutex.RUnlock()

	_, ok := manager.IsTestSuiteRunning(testSuite)
	if !ok {
		return ErrNoSuchTestSuite
	}
	if containerID == "simulation" {
		containerID = manager.simContainerID
	}

	networkID, exists := manager.networks[testSuite][networkName]
	if !exists {
		return ErrNetworkNotFound
	}
	return manager.backend.DisconnectContainer(containerID, networkID)
}

// EndTestSuite ends the test suite by writing the test suite results to the supplied
// stream and removing the test suite from the running list
func (manager *TestManager) EndTestSuite(testSuite TestSuiteID) error {
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()
	return manager.doEndSuite(testSuite)
}

func (manager *TestManager) doEndSuite(testSuite TestSuiteID) error {
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
	if suite.testDetailsFile != nil {
		suite.testDetailsFile.Close()
	}

	// Create comprehensive run metadata
	runMetadata := &RunMetadata{
		HiveCommand: manager.hiveInfo.Command,
		HiveVersion: GetHiveVersion(),
	}

	// Add client configuration if available
	if manager.hiveInfo.ClientFilePath != "" && len(manager.hiveInfo.ClientFile) > 0 {
		// Convert existing ClientFile data to consistent format for storage
		clientConfigContent := map[string]interface{}{
			"clients": manager.hiveInfo.ClientFile,
		}

		runMetadata.ClientConfig = &ClientConfigInfo{
			FilePath: manager.hiveInfo.ClientFilePath,
			Content:  clientConfigContent,
		}
	}

	// Attach metadata to suite
	suite.RunMetadata = runMetadata

	// Clean up any shared clients for this suite.
	if suite.ClientInfo != nil {
		for nodeID, clientInfo := range suite.ClientInfo {
			// Stop the container if it's still running.
			if clientInfo.wait != nil {
				slog.Info("cleaning up shared client", "suite", testSuite, "client", clientInfo.Name, "container", nodeID[:8])
				if err := manager.backend.DeleteContainer(clientInfo.ID); err != nil {
					slog.Error("could not stop shared client", "suite", testSuite, "container", nodeID[:8], "err", err)
				}
				clientInfo.wait()
				clientInfo.wait = nil
			}
		}
	}

	// Write the result.
	if manager.config.LogDir != "" {
		err := writeSuiteFile(suite, manager.config.LogDir)
		if err != nil {
			return err
		}
	}
	// remove the test suite's left-over docker networks.
	if errs := manager.PruneNetworks(testSuite); len(errs) > 0 {
		for _, err := range errs {
			slog.Error("could not remove network", "err", err)
		}
	}
	// Move the suite to results.
	delete(manager.runningTestSuites, testSuite)
	manager.results[testSuite] = suite
	return nil
}

// StartTestSuite starts a test suite and returns the context id
func (manager *TestManager) StartTestSuite(name string, description string) (TestSuiteID, error) {
	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()

	newSuiteID := TestSuiteID(manager.testSuiteCounter)

	var (
		testLogPath string
		testLogFile *os.File
	)
	if manager.config.LogDir != "" {
		testLogPath = fmt.Sprintf("details/%d-%s-%d.log", time.Now().Unix(), manager.simContainerID, newSuiteID)
		fp := filepath.Join(manager.config.LogDir, filepath.FromSlash(testLogPath))

		if err := os.MkdirAll(filepath.Dir(fp), 0755); err != nil {
			return 0, err
		}
		file, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return 0, err
		}
		testLogFile = file
	}

	manager.runningTestSuites[newSuiteID] = &TestSuite{
		ID:              newSuiteID,
		Name:            name,
		Description:     description,
		ClientVersions:  make(map[string]string),
		TestCases:       make(map[TestID]*TestCase),
		SimulatorLog:    manager.simLogFile,
		TestDetailsLog:  testLogPath,
		testDetailsFile: testLogFile,
	}
	manager.testSuiteCounter++
	return newSuiteID, nil
}

// StartTest starts a new test case, returning the testcase id as a context identifier
func (manager *TestManager) StartTest(testSuiteID TestSuiteID, name string, description string) (TestID, error) {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	// check if the testsuite exists
	testSuite, ok := manager.runningTestSuites[testSuiteID]
	if !ok {
		return 0, ErrNoSuchTestSuite
	}
	// increment the testcasecounter
	manager.testCaseCounter++
	var newCaseID = TestID(manager.testCaseCounter)
	// create a new test case and add it to the test suite
	newTestCase := &TestCase{
		Name:        name,
		Description: description,
		Start:       time.Now(),
	}
	// add the test case to the test suite
	testSuite.TestCases[newCaseID] = newTestCase
	// and to the general map of id:testcases
	manager.runningTestCases[newCaseID] = newTestCase

	return newCaseID, nil
}

// EndTest finishes the test case
func (manager *TestManager) EndTest(suiteID TestSuiteID, testID TestID, result *TestResult) error {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	// Check if the test case is running
	testSuite, ok := manager.runningTestSuites[suiteID]
	if !ok {
		return ErrNoSuchTestCase
	}
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}
	// Make sure there is at least a result summary
	if result == nil {
		return ErrNoSummaryResult
	}

	// Add the results to the test case
	testCase.End = time.Now()
	if result.Details != "" && testSuite.testDetailsFile != nil {
		offsets := manager.writeTestDetails(testSuite, testCase, result.Details)
		result.Details = ""
		result.LogOffsets = offsets
	}

	// Log the number of clients in the test case for debugging
	if len(testCase.ClientInfo) > 0 {
		slog.Debug("Processing client logs",
			"testID", testID,
			"clientCount", len(testCase.ClientInfo),
			"clientTypes", fmt.Sprintf("%v", func() []string {
				types := make([]string, 0, len(testCase.ClientInfo))
				for _, ci := range testCase.ClientInfo {
					types = append(types, ci.Name)
				}
				return types
			}()))
	} else {
		slog.Debug("No clients registered with test", "testID", testID)
	}

	for _, clientInfo := range testCase.ClientInfo {
		if clientInfo.IsShared {
			// Get current log position
			logEndByte, err := manager.getClientCurrentByteCount(clientInfo)
			if err != nil {
				slog.Error("could not get client log position", "err", err)
				continue
			}
			clientInfo.LogEndByte = &logEndByte
		}
	}

	testCase.SummaryResult = *result

	// Stop running clients that are not shared.
	for _, v := range testCase.ClientInfo {
		if v.wait != nil {
			manager.backend.DeleteContainer(v.ID)
			v.wait()
			v.wait = nil
		}
	}

	// Delete from running, if it's still there.
	delete(manager.runningTestCases, testID)
	return nil
}

func (manager *TestManager) writeTestDetails(suite *TestSuite, testCase *TestCase, text string) *TestLogOffsets {
	var (
		begin   = suite.testLogOffset
		header  = "-- " + testCase.Name + "\n"
		footer  = "\n\n"
		offsets TestLogOffsets
	)
	n, err := fmt.Fprint(suite.testDetailsFile, header, text, footer)
	suite.testLogOffset += int64(n)

	if err != nil {
		slog.Error("could not write details file", "err", err)
		// Write was incomplete, so play it safe with the offsets.
		offsets.Begin = begin
		offsets.End = begin + int64(n)
	} else {
		// Otherwise, exclude the header and footer in offsets.
		// They are just written to make the file more readable.
		offsets.Begin = begin + int64(len(header))
		offsets.End = offsets.Begin + int64(len(text))
	}
	return &offsets
}

// RegisterNode is used by test suite hosts to register the creation of a node.
// If testID is nil, the node is a shared client.
func (manager *TestManager) RegisterNode(suiteID TestSuiteID, testID *TestID, nodeID string, nodeInfo *ClientInfo) error {
	if testID != nil {
		manager.testCaseMutex.Lock()
		defer manager.testCaseMutex.Unlock()

		// Check if the test case is running
		testCase, ok := manager.runningTestCases[*testID]
		if !ok {
			return ErrNoSuchTestCase
		}
		if testCase.ClientInfo == nil {
			testCase.ClientInfo = make(map[string]*ClientInfo)
		}

		testCase.ClientInfo[nodeID] = nodeInfo
		return nil
	} else {
		manager.testSuiteMutex.Lock()
		defer manager.testSuiteMutex.Unlock()

		// Check if the test suite is running
		testSuite, ok := manager.runningTestSuites[suiteID]
		if !ok {
			return ErrNoSuchTestSuite
		}

		// Initialize shared clients map if it doesn't exist
		if testSuite.ClientInfo == nil {
			testSuite.ClientInfo = make(map[string]*ClientInfo)
		}

		testSuite.ClientInfo[nodeID] = nodeInfo
		return nil
	}
}

// RegisterSharedClient registers an already started shared client in a specific test
func (manager *TestManager) RegisterSharedClient(suiteID TestSuiteID, testID TestID, nodeID string) error {
	suite, ok := manager.runningTestSuites[suiteID]
	if !ok {
		return ErrNoSuchTestSuite
	}
	if suite.ClientInfo == nil {
		return ErrNoSuchNode
	}
	nodeInfo, ok := suite.ClientInfo[nodeID]
	if !ok {
		return ErrNoSuchNode
	}
	if nodeInfo.wait == nil {
		return ErrSharedClientNotRunning
	}
	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}
	if testCase.ClientInfo == nil {
		testCase.ClientInfo = make(map[string]*ClientInfo)
	}
	logStartByte, err := manager.getClientCurrentByteCount(nodeInfo)
	if err != nil {
		return err
	}
	logStartByte += 1

	// Create a new client info object with the shared client info,
	// without the wait function because we don't want to stop the
	// container at the end of the test
	nodeInfoTestCopy := ClientInfo{
		ID:             nodeInfo.ID,
		IP:             nodeInfo.IP,
		Name:           nodeInfo.Name,
		InstantiatedAt: nodeInfo.InstantiatedAt,
		LogFile:        nodeInfo.LogFile,
		IsShared:       nodeInfo.IsShared,
		LogStartByte:   &logStartByte,
		LogEndByte:     nil, // TODO: get the end position from the log file when the test is finished
	}
	// TODO: We can optionally add a stopClient flag to the RegisterSharedClient function
	// to stop the client at the end of the test, but we need to create a new wait function
	// that calls and then clears the wait function from the original nodeInfo.
	testCase.ClientInfo[nodeID] = &nodeInfoTestCopy

	return nil
}

// StopNode stops a client container.
func (manager *TestManager) StopNode(suiteID TestSuiteID, testID *TestID, nodeID string) error {
	var nodeInfo *ClientInfo
	if testID != nil {
		manager.testCaseMutex.Lock()
		defer manager.testCaseMutex.Unlock()

		testCase, ok := manager.runningTestCases[*testID]
		if !ok {
			return ErrNoSuchNode
		}
		nodeInfo, ok = testCase.ClientInfo[nodeID]
		if !ok {
			return ErrNoSuchNode
		}

	} else {
		manager.testSuiteMutex.Lock()
		defer manager.testSuiteMutex.Unlock()

		// Check if the test suite is running
		testSuite, ok := manager.runningTestSuites[suiteID]
		if !ok {
			return ErrNoSuchTestSuite
		}

		if testSuite.ClientInfo == nil {
			return ErrNoSuchNode
		}
		nodeInfo, ok = testSuite.ClientInfo[nodeID]
		if !ok {
			return ErrNoSuchNode
		}
	}
	// Stop the container.
	if nodeInfo.wait != nil {
		if err := manager.backend.DeleteContainer(nodeInfo.ID); err != nil {
			return fmt.Errorf("unable to stop client: %v", err)
		}
		nodeInfo.wait()
		nodeInfo.wait = nil
	}
	return nil
}

// PauseNode pauses a client container.
func (manager *TestManager) PauseNode(testID TestID, nodeID string) error {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchNode
	}
	nodeInfo, ok := testCase.ClientInfo[nodeID]
	if !ok {
		return ErrNoSuchNode
	}
	// Pause the container.
	if err := manager.backend.PauseContainer(nodeInfo.ID); err != nil {
		return fmt.Errorf("unable to pause client: %v", err)
	}
	return nil
}

// UnpauseNode unpauses a client container.
func (manager *TestManager) UnpauseNode(testID TestID, nodeID string) error {
	manager.testCaseMutex.Lock()
	defer manager.testCaseMutex.Unlock()

	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchNode
	}
	nodeInfo, ok := testCase.ClientInfo[nodeID]
	if !ok {
		return ErrNoSuchNode
	}
	// Unpause the container.
	if err := manager.backend.UnpauseContainer(nodeInfo.ID); err != nil {
		return fmt.Errorf("unable to unpause client: %v", err)
	}
	return nil
}

// countLinesInFile counts the current number of lines in a file (1-based).
func (manager *TestManager) getClientCurrentByteCount(clientInfo *ClientInfo) (int64, error) {
	// Ensure we have the full path to the log file
	fullPath := clientInfo.LogFile
	if !filepath.IsAbs(fullPath) {
		fullPath = filepath.Join(manager.config.LogDir, clientInfo.LogFile)
	}
	slog.Debug("Opening log file", "path", fullPath)

	fileInfo, err := os.Stat(fullPath)
	if err != nil {
		return 0, err
	}

	return fileInfo.Size(), nil
}

// writeSuiteFile writes the simulation result to the log directory.
// List of build arguments to exclude from result JSON for security/privacy
var excludedBuildArgs = map[string]bool{
	"GOPROXY":      true, // Go proxy URLs may contain sensitive info
	"GITHUB_TOKEN": true, // GitHub tokens
	"ACCESS_TOKEN": true, // Generic access tokens
	"API_KEY":      true, // API keys
	"PASSWORD":     true, // Passwords
	"SECRET":       true, // Generic secrets
	"TOKEN":        true, // Generic tokens
}

func writeSuiteFile(s *TestSuite, logdir string) error {
	suiteData, err := json.Marshal(s)
	if err != nil {
		return err
	}
	// Randomize the name, but make it so that it's ordered by date - makes cleanups easier
	b := make([]byte, 16)
	rand.Read(b)
	suiteFileName := fmt.Sprintf("%v-%x.json", time.Now().Unix(), b)
	suiteFile := filepath.Join(logdir, suiteFileName)
	// Write it.
	return os.WriteFile(suiteFile, suiteData, 0644)
}
