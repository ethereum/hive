package common

import (
	"sync"
	"time"
)

// TestManager offers providers a common implementation for
// managing tests. It is a partial implementation of
// the TestSuiteHost interface
type TestManager struct {
	OutputPath       string
	KillNodeCallback func(testSuite TestSuiteID, test TestID, node string) error

	runningTestSuites map[TestSuiteID]*TestSuite
	runningTestCases  map[TestID]*TestCase
	testCaseMutex     sync.Mutex
	testSuiteMutex    sync.Mutex
	nodeMutex         sync.Mutex
	testSuiteCounter  uint32
	testCaseCounter   uint32
}

// NewTestManager is a constructor returning a TestManager
func NewTestManager(outputPath string, killNodeCallback func(testSuite TestSuiteID, test TestID, node string) error) TestManager {
	return TestManager{
		OutputPath:        outputPath,
		KillNodeCallback:  killNodeCallback,
		runningTestSuites: make(map[TestSuiteID]*TestSuite),
		runningTestCases:  make(map[TestID]*TestCase),
	}
}

// IsTestSuiteRunning checks if the test suite is still running and returns it if so
func (manager *TestManager) IsTestSuiteRunning(testSuite TestSuiteID) (*TestSuite, bool) {
	suite, ok := manager.runningTestSuites[testSuite]
	return suite, ok
}

// IsTestRunning checks if the testis still running and returns it if so
func (manager *TestManager) IsTestRunning(test TestID) (*TestCase, bool) {
	testCase, ok := manager.runningTestCases[test]
	return testCase, ok
}

// GetNode gets some node info belonging to some tests
func (manager *TestManager) GetNode(testSuite TestSuiteID, test TestID, nodeID string) (*TestClientInfo, error) {
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
		return nil, ErrNoSuchNode
	}
	return nodeInfo, nil
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
func (manager *TestManager) StartTestSuite(name string, description string) (TestSuiteID, error) {

	manager.testSuiteMutex.Lock()
	defer manager.testSuiteMutex.Unlock()
	var newSuiteID = TestSuiteID(manager.testSuiteCounter)
	manager.runningTestSuites[newSuiteID] = &TestSuite{
		ID:          newSuiteID,
		Name:        name,
		Description: description,
		TestCases:   make(map[TestID]*TestCase),
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
	defer manager.testCaseMutex.Unlock()
	// Check if the test case is running
	testCase, ok := manager.runningTestCases[testID]
	if !ok {
		return ErrNoSuchTestCase
	}
	// Make sure there is at least a result summary
	if summaryResult == nil {
		return ErrNoSummaryResult
	}
	// Add the results to the test case
	testCase.End = time.Now()
	testCase.SummaryResult = *summaryResult
	testCase.ClientResults = clientResults
	delete(manager.runningTestCases, testCase.ID)
	for k := range testCase.ClientInfo {
		manager.KillNodeCallback(testSuiteRun, testID, k)
	}

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

//TODO - consider RegisterPseudo
