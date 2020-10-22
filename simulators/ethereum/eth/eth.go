package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ethereum/hive/simulators/common/providers/hive"
)

var tests = []hive.SingleClientTest{{
	Name:        "eth protocol",
	Description: "This tests a client's ability to accurately respond to basic eth protocol messages.",
	Run:         runEthTest,
	Parameters: map[string]string{
		"HIVE_NETWORK_ID":     "19763",
		"HIVE_CHAIN_ID":       "19763",
		"HIVE_FORK_HOMESTEAD": "0",
		"HIVE_FORK_TANGERINE": "0",
		"HIVE_FORK_SPURIOUS":  "0",
		"HIVE_FORK_BYZANTIUM": "0",
		"HIVE_LOGLEVEL":       "5",
	},
	Files: map[string]string{
		"genesis.json": "/init/genesis.json",
		"chain.rlp":    "/init/halfchain.rlp",
	},
}}

func main() {
	host := hive.New()
	err := hive.RunAllClients(host, "eth protocol test suite", tests)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runEthTest(t *hive.ClientTest) {
	enode, err := t.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("./devp2p", "rlpx", "eth-test", enode, "/init/fullchain.rlp", "/init/genesis.json")
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
