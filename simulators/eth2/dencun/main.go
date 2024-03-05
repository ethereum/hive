package main

import (
	"os"
	"runtime/pprof"

	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
	suite_builder "github.com/ethereum/hive/simulators/eth2/dencun/suites/builder"
	suite_blobs_gossip "github.com/ethereum/hive/simulators/eth2/dencun/suites/p2p/gossip/blobs"
	suite_reorg "github.com/ethereum/hive/simulators/eth2/dencun/suites/reorg"
	suite_sync "github.com/ethereum/hive/simulators/eth2/dencun/suites/sync"
)

func main() {
	// Create a CPU profile file
	cpuProfileFile, err := os.Create("cpu.prof")
	if err != nil {
		panic(err)
	}
	defer cpuProfileFile.Close()

	// Start CPU profiling
	if err := pprof.StartCPUProfile(cpuProfileFile); err != nil {
		panic(err)
	}
	defer pprof.StopCPUProfile()

	// Create simulator that runs all tests
	sim := hivesim.New()
	if sim == nil {
		panic("failed to create simulator")
	}
	// From the simulator we can get all client types provided
	clientTypes, err := sim.ClientTypes()
	if err != nil {
		panic(err)
	}
	clientsByRole := clients.ClientsByRole(clientTypes)
	if clientsByRole == nil {
		panic("failed to create clients by role")
	}

	// Mark suites for execution
	hivesim.MustRunSuite(sim, suite_base.Suite(clientsByRole))
	hivesim.MustRunSuite(sim, suite_sync.Suite(clientsByRole))
	hivesim.MustRunSuite(sim, suite_builder.Suite(clientsByRole))
	hivesim.MustRunSuite(sim, suite_reorg.Suite(clientsByRole))
	hivesim.MustRunSuite(sim, suite_blobs_gossip.Suite(clientsByRole))
}
