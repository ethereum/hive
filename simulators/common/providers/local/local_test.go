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
		]
	}`

	configBytes := []byte(config)

	err := generateInstance(configBytes)
	if err != nil {
		t.Errorf("Error executing get instance %s", err.Error())
	}
	if hostProxy == nil {
		t.Errorf("No test suite host returned")
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
		t.Errorf("Error executing get instance %s", err.Error())
	}
	if host == nil {
		t.Errorf("No test suite host returned")
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
		t.Errorf("Wrong configuration")
	}

	if hostProxy.configuration.AvailableClients[0].IP.String() != "10.3.58.6" ||
		hostProxy.configuration.AvailableClients[0].IsPseudo != false ||
		hostProxy.configuration.AvailableClients[0].Enode != "enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_DAO_BLOCK"] != "0" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_CONSTANTINOPLE_BLOCK"] != "0" ||
		hostProxy.configuration.AvailableClients[0].Mac != "00:0a:95:9d:68:16" ||
		hostProxy.configuration.AvailableClients[0].ClientType != "go-ethereum_master" {
		t.Errorf("Wrong configuration")
	}
}

func TestStartTestSuite(t *testing.T) {

	testSuiteHost := setupBasicInstance(t)

	suite1ID := testSuiteHost.StartTestSuite("consensus", "consensus tests")
	suite2ID := testSuiteHost.StartTestSuite("p2p", "p2p tests")

	suite1, ok1 := hostProxy.runningTestSuites[suite1ID]
	suite2, ok2 := hostProxy.runningTestSuites[suite2ID]

	if ok1 == false || ok2 == false {
		t.Errorf("Test suites not registered as running")
	}

	if suite1.Name != "consensus" || suite2.Name != "p2p" {
		t.Errorf("Test suite names not registered")
	}

	if suite1.Description != "consensus tests" || suite2.Description != "p2p tests" {
		t.Errorf("Test suite description not registered")
	}
}

func TestGetClientEnode(t *testing.T) {

	//testSuiteHost := setupBasicInstance(t)

}
