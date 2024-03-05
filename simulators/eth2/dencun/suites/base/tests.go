package suite_base

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-testnet",
	DisplayName: "Deneb Testnet",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb.`,
	Location:    "suites/base",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		BaseTestSpec{
			Name:         "test-deneb-fork",
			DisplayName:  "Deneb Fork",
			Description:  `Sanity test to check the fork transition to deneb.`,
			DenebGenesis: false,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			EpochsAfterFork:     1,
			ExitValidatorsShare: 10,
		},
		BaseTestSpec{
			Name:        "test-deneb-genesis",
			DisplayName: "Deneb Genesis",
			Description: `
			Sanity test to check the beacon clients can start with deneb genesis.
			`,
			DenebGenesis: true,
			GenesisExecutionWithdrawalCredentialsShares: 1,
			WaitForFinality:     true,
			ExitValidatorsShare: 10,
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
