package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ethereum/hive/simulators/common"

	"github.com/ethereum/hive/simulators/common/providers/hive"
)

var (
	host     common.TestSuiteHost
	suiteID  common.TestSuiteID
	testID   common.TestID
	clientID string
)

func main() {
	host = hive.New()

	// get clients
	// loop over individual clients
	// get node
	// start test

	clients, err := host.GetClientTypes()
	if err != nil {
		fatalf("could not get clients: %v", err)
	}

	logFile := os.Getenv("HIVE_SIMLOG")

	for _, client := range clients {
		env := map[string]string{
			"CLIENT":              client,
			"HIVE_NETWORK_ID":     "19763",
			"HIVE_CHAIN_ID":       "19763",
			"HIVE_FORK_HOMESTEAD": "0",
			"HIVE_FORK_TANGERINE": "0",
			"HIVE_FORK_SPURIOUS":  "0",
			"HIVE_FORK_BYZANTIUM": "0",
			"HIVE_LOGLEVEL":       "5",
		}

		var err error
		suiteID, err = host.StartTestSuite("discv4", "discovery v4 test", logFile)
		if err != nil {
			fatalf("could not start test suite: %v", err)
		}

		testID, err = host.StartTest(suiteID, "discovery v4", "This test suite tests a "+
			"client's conformance to the discovery v4 protocol.")
		if err != nil {
			fatalf("could not start test: %v", err)
		}
		// get container ID and bridge IP of the client
		clientID, _, _, err := host.GetNode(suiteID, testID, env, nil) // TODO can pass nil for files?
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("error getting node: %v", err)
		}
		// get enode ID from client
		enode, err := host.GetClientEnode(suiteID, testID, clientID)
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("error getting node peer-to-peer endpoint: %v", err)
		}
		// create a separate network to be able to send the client traffic from two separate IP addrs
		networkID, err := host.CreateNetwork(suiteID, "network1")
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("could not create network: %v", err)
		}
		// connect both simulation and client to this network
		err = host.ConnectContainer(suiteID, networkID, clientID)
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("could not connect container to network: %v", err)
		}
		err = host.ConnectContainer(suiteID, networkID, "simulation")
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("could not connect container to network: %v", err)
		}
		// get bridge network ID
		bridgeID, err := host.GetNetworkID(suiteID, "bridge")
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("could not get ID of network: %v", err)
		}
		// get client IP from bridge and network1
		simIPBridge, err := host.GetContainerNetworkIP(suiteID, bridgeID, "simulation")
		simIPNetwork1, err := host.GetContainerNetworkIP(suiteID, networkID, "simulation")
		if err != nil {
			endTest(&common.TestResult{
				Pass:    false,
				Details: err.Error(),
			})
			fatalf("could not connect container IP from network: %v", err)
		}
		// run the discovery test
		err = runDiscTest(*enode, simIPNetwork1, simIPBridge)
		result := &common.TestResult{Pass: err == nil}
		if err != nil {
			result.Details = err.Error()
		}
		endTest(result)
	}
}

func runDiscTest(enode, networkIP, bridgeIP string) error {
	cmd := exec.Command("./devp2p", "discv4", "test", "-remote", enode, "-listen1", bridgeIP, "-listen2", networkIP)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func endTest(result *common.TestResult) {
	host.KillNode(suiteID, testID, clientID)
	host.EndTest(suiteID, testID, result, nil)
	host.EndTestSuite(suiteID)
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Errorf(format, args...))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
