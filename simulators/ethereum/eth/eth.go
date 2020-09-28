package main

import (
	"fmt"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"os"
	"os/exec"

	"github.com/ethereum/hive/simulators/common"
)

func main() {
	host, err := hive.New(os.Getenv("HIVE_SIMULATOR"))
	if err != nil {
		fatal(fmt.Errorf("unable to initialise provider %s", err.Error()))
	}

	//	get clients
	// for loop that runs tests against each client type
	// get node
	// start test
	// run tests against node
	// end test

	files := map[string]string{
		"genesis.json": "/init/genesis.json",
		"chain.rlp":    "/init/halfchain.rlp",
	}

	clients, err := host.GetClientTypes()
	if err != nil {
		fatal(fmt.Errorf("could not get clients: %v", err))
	}

	logFile := os.Getenv("HIVE_SIMLOG")

	for _, client := range clients {
		env := map[string]string{
			"CLIENT":              client,
			"HIVE_NETWORK_ID":     "1",
			"HIVE_CHAIN_ID":       "1",
			"HIVE_FORK_HOMESTEAD": "0",
			"HIVE_FORK_TANGERINE": "0",
			"HIVE_FORK_SPURIOUS":  "0",
			"HIVE_FORK_BYZANTIUM": "0",
			"HIVE_LOGLEVEL":       "5",
		}

		suite, err := host.StartTestSuite("eth", "eth protocol test", logFile)
		if err != nil {
			fatalf("could not start test suite: %v", err)
		}
		testID, err := host.StartTest(suite, "eth protocol test", "This test suite manually tests a " +
			"client's ability to accurately respond to basic eth protocol messages.")
		if err != nil {
			fatalf("could not start test: %v", err)
		}
		containerID, _, _, err := host.GetNode(suite, testID, env, files)
		if err != nil {
			fatalf("error getting node: %v", err)
		}
		enode, err := host.GetClientEnode(suite, testID, containerID)
		if err != nil {
			fatalf("error getting node peer-to-peer endpoint: %v", err)
		}

		err = runEthTest(*enode)
		result := &common.TestResult{Pass: err == nil}
		if err != nil {
			result.Details = err.Error()
		}

		host.KillNode(suite, testID, containerID)
		host.EndTest(suite, testID, result, nil)
		host.EndTestSuite(suite)
	}
}

func runEthTest(enode string) error {
	cmd := exec.Command("./devp2p", "rlpx", "eth-test", enode, "/init/fullchain.rlp", "/init/genesis.json")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Errorf(format, args...))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
