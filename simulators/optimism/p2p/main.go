package main

import (
	"context"
	"time"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/optimism"
)

func main() {
	suite := hivesim.Suite{
		Name:        "optimism p2p",
		Description: "This suite runs the P2P protocol tests",
	}

	// Add tests for full nodes.
	suite.Add(&hivesim.TestSpec{
		Name:        "client launch",
		Description: `This test launches the client and collects its logs.`,
		Run:         func(t *hivesim.T) { runP2PTests(t) },
	})

	sim := hivesim.New()
	hivesim.MustRunSuite(sim, suite)
}

// runP2PTests runs the P2P tests between the sequencer and verifier.
func runP2PTests(t *hivesim.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	d := optimism.Devnet{
		T:     t,
		Nodes: make(map[string]*hivesim.ClientDefinition),
		Ctx:   ctx,
	}
	d.Start()
	d.Wait()
	d.DeployL1()
	d.InitL2()
	d.StartL2()
	d.StartOp()
	d.StartVerifier()
	d.StartL2OS()
	d.StartBSS()

	// TODO: Add P2P tests
}
