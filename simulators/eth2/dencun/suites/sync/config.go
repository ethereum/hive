package suite_sync

import (
	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
)

type SyncTestSpec struct {
	suite_base.BaseTestSpec
}

func (ts SyncTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	tc := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	// We disable the start of the last node
	tc.NodeDefinitions[len(tc.NodeDefinitions)-1].DisableStartup = true

	return tc
}
