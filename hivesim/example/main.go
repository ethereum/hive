package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/ethereum/hive/hivesim"
)

func main() {}

// -------------------------------------------------------------------------------
// --- Sync Test (generated tests based on available clients, overlapping tests)

func syncTest() {
	sim := hivesim.New()
	clientTypes, err := sim.GetClientTypes()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

}

func runSourceTest(t *hivesim.T, source *hivesim.Client) {
	// Check chain head.

	t.RunClientTest(hivesim.ClientTestSpec{
		Name:        fmt.Sprintf("sync from %s", source.Type),
		Description: fmt.Sprintf("This runs a sync against the %s source node.", source.Type),
		Run:         runSyncTest,
	})
}

// -------------------------------------------------------------------------------
// --- Single Client

func singleClient() {
	var suite = hivesim.Suite{
		Name:        "eth protocol",
		Description: "This tests a client's ability to ...",
	}
	suite.Add(hivesim.ClientTestSpec{
		Parameters: map[string]string{
			"HIVE_NETWORK_ID":     "1",
			"HIVE_CHAIN_ID":       "1",
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
	})

	sim := hivesim.New()
	err := hivesim.RunSuite(sim, suite)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func runEthTest(t *hivesim.T, c *hivesim.Client) {
	enode, err := c.EnodeURL()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("./devp2p", "rlpx", "eth-test", enode, "/init/fullchain.rlp", "/init/genesis.json")
	// TODO: maybe add API to pipe stuff into test output?
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
}
