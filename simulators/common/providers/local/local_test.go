package local

import (
	"testing"

	"github.com/ethereum/hive/simulators/common"
)

// avoids the singleton
func setupBasicInstance(t *testing.T) common.TestSuiteHost {
	var config = `
	{
		"availableClients": [
			{
				"clientType":"go-ethereum_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.6",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:16",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"0",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"0"
				}
			}
		,
		
			{
				"clientType":"go-ethereum_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.0",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:00",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"0",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"10"
				}
			}
		,
		
			{
				"clientType":"nethermind_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.7",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:17",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"1",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"1"
				}
			}
		,
		
			{
				"clientType":"parity_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.8",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:18",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"2",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"2"
				}
			}
			,
			{
				"clientType":"relay",
				"ip":"10.3.58.99",
				"isPseudo":true,
				"mac":"00:0a:95:9d:68:99"
			}
		],
		"outputFile":"c:\mytests\newtests\testResults.json"
	}`

	configBytes := []byte(config)

	err := generateInstance(configBytes)
	if err != nil {
		t.Fatalf("Error executing get instance %s", err.Error())
	}
	if hostProxy == nil {
		t.Fatalf("No test suite host returned")
	}
	return hostProxy
}

func TestGetInstance(t *testing.T) {

	var config = `
	{
		"availableClients": [
			{
				"clientType":"go-ethereum_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.6",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:16",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"0",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"0"
				}
			}
		]
	}`

	configBytes := []byte(config)

	host, err := GetInstance(configBytes)

	if err != nil {
		t.Fatalf("Error executing get instance %s", err.Error())
	}
	if host == nil {
		t.Fatalf("No test suite host returned")
	}

	config = `
	{
		"availableClients": [
			{
				"clientType":"parity_master",
				"enode":"enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301",
				"ip":"10.3.58.6",
				"isPseudo":false,
				"mac":"00:0a:95:9d:68:16",
				"parameters":{
					"HIVE_FORK_DAO_BLOCK":"0",
					"HIVE_FORK_CONSTANTINOPLE_BLOCK":"0"
				}
			}
		]
	}`
	configBytes = []byte(config)
	GetInstance(configBytes)

	//check the underlying unexported instance
	if hostProxy == nil || hostProxy.configuration == nil || len(hostProxy.configuration.AvailableClients) != 1 {
		t.Fatalf("Wrong configuration")
	}

	if hostProxy.configuration.AvailableClients[0].IP.String() != "10.3.58.6" ||
		hostProxy.configuration.AvailableClients[0].IsPseudo != false ||
		*hostProxy.configuration.AvailableClients[0].Enode != "enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_DAO_BLOCK"] != "0" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_CONSTANTINOPLE_BLOCK"] != "0" ||
		*hostProxy.configuration.AvailableClients[0].Mac != "00:0a:95:9d:68:16" ||
		hostProxy.configuration.AvailableClients[0].ClientType != "go-ethereum_master" {
		t.Fatalf("Wrong configuration")
	}
}

func TestStartTestSuite(t *testing.T) {

	testSuiteHost := setupBasicInstance(t)

	suite1ID := testSuiteHost.StartTestSuite("consensus", "consensus tests")
	suite2ID := testSuiteHost.StartTestSuite("p2p", "p2p tests")

	suite1, ok1 := hostProxy.runningTestSuites[suite1ID]
	suite2, ok2 := hostProxy.runningTestSuites[suite2ID]

	if ok1 == false || ok2 == false {
		t.Fatalf("Test suites not registered as running")
	}

	if suite1.Name != "consensus" || suite2.Name != "p2p" {
		t.Fatalf("Test suite names not registered")
	}

	if suite1.Description != "consensus tests" || suite2.Description != "p2p tests" {
		t.Fatalf("Test suite description not registered")
	}
}

func setupTestSuite(t *testing.T) (common.TestSuiteHost, *common.TestSuite) {
	testSuiteHost := setupBasicInstance(t)
	suiteID := testSuiteHost.StartTestSuite("consensus", "consensus tests")
	suite := hostProxy.runningTestSuites[suiteID]
	if suite == nil {
		t.Fatalf("Test setup failed, test suite not found")
	}
	return testSuiteHost, suite
}

func TestStartTest(t *testing.T) {

	//just rely on there being a working StartTestSuite as set up:
	host, suite := setupTestSuite(t)

	randomID := suite.ID + 10

	_, err := host.StartTest(randomID, "notest", "notest description")
	if err == nil {
		t.Fatalf("Test was started without a valid test suite context")
	}

	testCase, err := host.StartTest(suite.ID, "atest", "atest description")
	if err != nil {
		t.Fatalf("Test creation failed: %s", err.Error())
	}

	tcase, ok := suite.TestCases[testCase]
	if !ok {
		t.Fatalf("Test creation failed: test does not exist")
	}

	if tcase.Name != "atest" || tcase.Description != "atest description" {
		t.Fatalf("Test has incorrect data")
	}

	if tcase.Start.IsZero() {
		t.Fatalf("Test has missing start time")
	}

}

func TestGetNode(t *testing.T) {
	// just use the suite and case functions for set up
	host, suite := setupTestSuite(t)

	testCaseID, err := host.StartTest(suite.ID, "notest", "notest description")
	if err != nil {
		t.Fatalf("Test setup failed: testCase could not be created.")
	}

	// test that it gets a node and that parameter selection is used
	parms := map[string]string{
		"CLIENT":                         "go-ethereum_master",
		"HIVE_FORK_CONSTANTINOPLE_BLOCK": "10",
	}
	_, _, mac, err := host.GetNode(testCaseID, parms)
	if err != nil {
		t.Fatalf("Unable to get a node from pre-supplied list: %s", err.Error())
	}

	// test that the node matches the requested client type
	if *mac != "00:0a:95:9d:68:00" {
		t.Fatalf("Incorrect node supplied")
	}

	// test that the node selected is the least used
	parms = map[string]string{
		"CLIENT": "go-ethereum_master",
	}
	_, _, mac, err = host.GetNode(testCaseID, parms)
	if *mac != "00:0a:95:9d:68:16" {
		t.Fatalf("Incorrect node supplied getting least used")
	}

}

func TestGetClientEnode(t *testing.T) {

	// just use the suite and case functions for set up
	host, suite := setupTestSuite(t)
	testCaseID, err := host.StartTest(suite.ID, "notest", "notest description")
	if err != nil {
		t.Fatalf("Test setup failed: testCase could not be created.")
	}
	parms := map[string]string{
		"CLIENT":                         "go-ethereum_master",
		"HIVE_FORK_CONSTANTINOPLE_BLOCK": "10",
	}
	nodeID, _, _, err := host.GetNode(testCaseID, parms)
	if err != nil {
		t.Fatalf("Test setup failed: unable to get a node from pre-supplied list: %s", err.Error())
	}

	enode, err := host.GetClientEnode(testCaseID, nodeID)
	if err != nil {
		t.Fatalf("Error in GetClientEnode %s", err.Error())
	}
	if *enode != "enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301" {
		t.Fatalf("Incorrect enode")
	}
}

func TestGetPseudo(t *testing.T) {
	// just use the suite and case functions for set up
	host, suite := setupTestSuite(t)

	testCaseID, err := host.StartTest(suite.ID, "notest", "notest description")
	if err != nil {
		t.Fatalf("Test setup failed: testCase could not be created.")
	}

	parms := map[string]string{
		"CLIENT": "relay",
	}
	_, _, mac, err := host.GetPseudo(testCaseID, parms)
	if err != nil {
		t.Fatalf("Error getting pseudo %s", err.Error())
	}
	if *mac != "00:0a:95:9d:68:99" {
		t.Fatalf("Incorrect node supplied getting least used")
	}
}

func TestGetClientTypes(t *testing.T) {
	host := setupBasicInstance(t)

	actualTypes, err := host.GetClientTypes()
	if err != nil {

	}

	expectedTypes := []string{
		"go-ethereum_master",
		"nethermind_master",
		"parity_master",
		"relay",
	}

	if len(expectedTypes) != len(actualTypes) {
		t.Fatalf("Incorrect number of types returned")
	}

	for _, ty := range expectedTypes {
		found := false
		for _, a := range actualTypes {
			if ty == a {
				found = true
			}
		}
		if !found {
			t.Fatalf("Expected client type missing %s", ty)
		}
	}
}
