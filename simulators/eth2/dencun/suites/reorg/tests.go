package suite_reorg

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-reorg",
	DisplayName: "Deneb Reorg",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb and test re-orgs of the beacon chain.`,
	Location:    "suites/reorg",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		ReorgTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-reorg-from-capella",
				Description: `
				Test re-org of the beacon chain by a third non-validating client, re-org from capella.
				Start two clients disconnected from each other, then connect them through a third client and check that they re-org.
				`,
				NodeCount: 5,
				// Wait for 1 epoch after the fork to start the last client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
			},
			// Chain A has 66% of the validators, chain B has 33% of the validators
			ChainWeights: []uint64{2, 1},
		},
		ReorgTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name: "test-reorg-from-deneb",
				Description: `
				Test re-org of the beacon chain by a third non-validating client, re-org from deneb.
				Start two clients disconnected from each other, then connect them through a third client and check that they re-org.
				`,
				NodeCount: 5,
				// Wait for 1 epoch after the fork to start the last client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
				DenebGenesis: true,
			},
			// Chain A has 66% of the validators, chain B has 33% of the validators
			ChainWeights: []uint64{2, 1},
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
