package main

import (
	"os"
	"os/exec"

	"github.com/fjl/hiveclient/hivesim"
)

var suite = hivesim.Suite{
	Name:        "eth protocol",
	Description: "This tests a client's ability to accurately respond to basic eth protocol messages.",
}

var test = hivesim.ClientTestSpec{
	Parameters: hivesim.Params{
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
	Run: runEthTest,
}

func main() {
	suite.Add(test)
	host := hivesim.New()
	hivesim.MustRunSuite(host, suite)
}

func runEthTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
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
