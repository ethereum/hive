package local

import (
	"testing"
)

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

	testSuiteHost, err := GetInstance(configBytes)

	if err != nil {
		t.Errorf("Error executing get instance %s", err.Error())
	}

	if testSuiteHost == nil {
		t.Errorf("No test suite host returned")
	}

	//check the underlying unexported instance
	if hostProxy == nil || hostProxy.configuration == nil || len(hostProxy.configuration.AvailableClients) != 1 {
		t.Errorf("Wrong configuration")
	}

	if hostProxy.configuration.AvailableClients[0].IP.String() != "10.3.58.6" ||
		hostProxy.configuration.AvailableClients[0].IsPseudo != false ||
		hostProxy.configuration.AvailableClients[0].Enode != "enode://6f8a80d14311c39f35f516fa664deaaaa13e85b2f7493f37f6144d86991ec012937307647bd3b9a82abe2974e1407241d54947bbb39763a4cac9f77166ad92a0@10.3.58.6:30303?discport=30301" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_DAO_BLOCK"] != "0" ||
		hostProxy.configuration.AvailableClients[0].Parameters["HIVE_FORK_CONSTANTINOPLE_BLOCK"] != "0" ||
		hostProxy.configuration.AvailableClients[0].Mac != "00:0a:95:9d:68:16" {
		t.Errorf("Wrong configuration")
	}

}
