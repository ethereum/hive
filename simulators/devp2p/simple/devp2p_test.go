package main

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
)

func init() {
	hive.Support()
	local.Support()
}

func TestDiscovery(t *testing.T) {
	simProviderType := flag.String("simProvider", "", "the simulation provider type (local|hive)")
	providerConfigFile := flag.String("providerConfig", "", "the config json file for the provider")

	flag.Parse()
	host, err := common.InitProvider(*simProviderType, *providerConfigFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to initialise provider %s", err.Error())
		os.Exit(1)
	}

	client := "go-ethereum"
	testName := "discv4"
	testDescription := "Test 'em all"
	logFile, _ := os.LookupEnv("HIVE_SIMLOG")
	//start the test suite
	testSuite, err := host.StartTestSuite("Devp2p discovery v4 test suite",
		"This suite of tests checks for basic conformity to the discovery v4 protocol and for some known security weaknesses.",
		logFile)
	if err != nil {
		t.Fatalf("Simulator error. Failed to start test suite. %v ", err)
	}
	defer host.EndTestSuite(testSuite)
	testID, err := host.StartTest(testSuite, testName, testDescription)
	if err != nil {
		t.Fatalf("Unable to start test: %s", err.Error())
	}
	// declare empty test results
	var summaryResult common.TestResult
	var clientResults map[string]*common.TestResult
	//make sure the test ends
	defer func() {
		host.EndTest(testSuite, testID, &summaryResult, clientResults)
		//send test failures to standard outputs too
		if !summaryResult.Pass {
			t.Errorf("Test failed %s", summaryResult.Details)
		}
	}()
	// get a client node
	parms := map[string]string{
		"CLIENT":        client,
		"HIVE_BOOTNODE": "enode://158f8aab45f6d19c6cbf4a089c2670541a8da11978a2f90dbf6a502a4a3bab80d288afdbeb7ec0ef6d92de563767f3b1ea9e8e334ca711e9f8e2df5a0385e8e6@1.2.3.4:30303",
	}
	nodeID, ipAddr, macAddr, err := host.GetNode(testSuite, testID, parms, nil)
	if err != nil {
		summaryResult.Pass = false
		summaryResult.AddDetail(fmt.Sprintf("Unable to get node: %s", err.Error()))
		return
	}

	enodeID, err := host.GetClientEnode(testSuite, testID, nodeID)
	if err != nil {
		summaryResult.Pass = false
		summaryResult.AddDetail(fmt.Sprintf("Unable to get enode: %s", err.Error()))
		return
	}
	if enodeID == nil || *enodeID == "" {
		summaryResult.Pass = false
		summaryResult.AddDetail(fmt.Sprintf("Unable to get enode - client reported blank enodeID"))

	}
	targetNode, err := enode.ParseV4(*enodeID)
	if err != nil {
		summaryResult.Pass = false
		summaryResult.AddDetail(fmt.Sprintf("Unable to get enode: %s", err.Error()))
		return
	}

	if targetNode == nil {
		summaryResult.Pass = false
		summaryResult.AddDetail(fmt.Sprintf("Unable to generate targetNode %s", err.Error()))
		return
	}

	//replace the ip with what docker says it is
	targetNode = enode.NewV4(targetNode.Pubkey(), ipAddr, targetNode.TCP(), 30303)

	t.Log("TargetNode", targetNode.URLv4())
	/*
		resultMessage, ok := testFunc(t, targetNode)
		summaryResult.Pass = ok
		summaryResult.AddDetail(resultMessage)
	*/

}
