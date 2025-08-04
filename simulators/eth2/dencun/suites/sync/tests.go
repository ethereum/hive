package suite_sync

import (
	"github.com/ethereum/hive/hivesim"
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/dencun/suites"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
)

var testSuite = hivesim.Suite{
	Name:        "eth2-deneb-sync",
	DisplayName: "Deneb Sync",
	Description: `Collection of test vectors that use a ExecutionClient+BeaconNode+ValidatorClient testnet for Cancun+Deneb and test syncing of the beacon chain.`,
	Location:    "suites/sync",
}

var Tests = make([]suites.TestSpec, 0)

func init() {
	Tests = append(Tests,
		SyncTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-sync-from-capella",
				DisplayName: "Sync From Capella Transition",
				Description: `
				Test syncing of the beacon chain by a secondary non-validating client, sync from capella.

				- Start two validating nodes that begin on Capella/Shanghai genesis
				- Deneb/Cancun transition occurs on Epoch 1
				- Total of 128 Validators, 64 for each validating node
				- Wait for Deneb fork and start sending blob transactions to the Execution client
				- Wait one more epoch for the chain to progress and include blobs
				- Start a third client with the first two validating nodes as bootnodes
				- Wait for the third client to reach the head of the canonical chain
				- Verify on the consensus client on the synced client that:
				  - For each blob transaction on the execution chain, the blob sidecars are available for the
					beacon block at the same height
				  - The beacon block lists the correct commitments for each blob
				  - The blob sidecars and kzg commitments match on each block for the synced client
				`,
				NodeCount:           3,
				ValidatingNodeCount: 2,
				// Wait for 1 epoch after the fork to start the syncing client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
			},
		},
		SyncTestSpec{
			BaseTestSpec: suite_base.BaseTestSpec{
				Name:        "test-sync-from-deneb",
				DisplayName: "Sync From Deneb Genesis",
				Description: `
				Test syncing of the beacon chain by a secondary non-validating client, sync from deneb.

				- Start two validating nodes that begin on Deneb genesis
				- Total of 128 Validators, 64 for each validating node
				- Start sending blob transactions to the Execution client
				- Wait one epoch for the chain to progress and include blobs
				- Start a third client with the first two validating nodes as bootnodes
				- Wait for the third client to reach the head of the canonical chain
				- Verify on the consensus client on the synced client that:
				  - For each blob transaction on the execution chain, the blob sidecars are available for the
					beacon block at the same height
				  - The beacon block lists the correct commitments for each blob
				  - The blob sidecars and kzg commitments match on each block for the synced client
				`,
				NodeCount:           3,
				ValidatingNodeCount: 2,
				// Wait for 1 epoch after the fork to start the syncing client
				EpochsAfterFork: 1,
				// All validators start with BLS withdrawal credentials
				GenesisExecutionWithdrawalCredentialsShares: 0,
				DenebGenesis: true,
			},
		},
	)
}

func Suite(c *clients.ClientDefinitionsByRole) hivesim.Suite {
	suites.SuiteHydrate(&testSuite, c, Tests)
	return testSuite
}
