package main

import (
	"github.com/ethereum/hive/hivesim"
	auth "github.com/ethereum/hive/simulators/ethereum/engine/suites/auth"
	cancun "github.com/ethereum/hive/simulators/ethereum/engine/suites/cancun"
	excap "github.com/ethereum/hive/simulators/ethereum/engine/suites/exchange_capabilities"
	paris "github.com/ethereum/hive/simulators/ethereum/engine/suites/paris"
	shanghai "github.com/ethereum/hive/simulators/ethereum/engine/suites/shanghai"
)

func main() {
	simulator := hivesim.New()

	// Mark suites for execution
	hivesim.MustRunSuite(simulator, auth.Suite)
	hivesim.MustRunSuite(simulator, excap.Suite)
	// hivesim.MustRunSuite(simulator, suite_sync.Suite(simulator))
	hivesim.MustRunSuite(simulator, paris.Suite)
	hivesim.MustRunSuite(simulator, shanghai.Suite)
	hivesim.MustRunSuite(simulator, cancun.Suite)
}
