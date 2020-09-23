package main

import (
	"fmt"
	"os/exec"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/hive/simulators/common"
	"github.com/ethereum/hive/simulators/common/providers/hive"
	"github.com/ethereum/hive/simulators/common/providers/local"
	"os"
)

func init() {
	hive.Support()
	local.Support()
}

func main() {
	host, err := common.InitProvider("hive", "")
	if err != nil {
		log.Error(fmt.Sprintf("Unable to initialise provider %s", err.Error()))
		os.Exit(1)
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
		log.Error("could not get clients", "error: ", err)
		return
	}

	logFile, _ := os.LookupEnv("HIVE_SIMLOG")

	for _, client := range clients {
		env := map[string]string{
			"CLIENT": client,
		}

		suite, err := host.StartTestSuite("eth", "eth protocol test", logFile)
		if err != nil {
			log.Error("could not start test suite", "error: ", err)
			return
		}

		testID, err := host.StartTest(suite, "eth protocol test", "TODO!!!!!!!") // TODO description
		if err != nil {
			log.Error("could not start test", "error: ", err)
			return
		}

		containerID, _, _, err := host.GetNode(suite, testID, env, files)
		if err != nil {
			log.Error("error getting node", "err", err)
			return
		}

		enode, err := host.GetClientEnode(suite, testID, containerID)
		if err != nil {
			log.Error("error getting enode id", "err", err)
			return
		}

		err = runEthTest(*enode)
		if err != nil {
			log.Error("error running eth test", "node", client, "err", err)
			return
		}

		host.EndTestSuite(suite)
	}
}

func runEthTest(enode string) error {
	cmd := exec.Command("devp2p", "rlpx", "eth-test", enode, "/init/genesis.json", "/init/fullchain.rlp")
	return cmd.Run()
}
