package suite_reorg

import (
	"fmt"

	"github.com/ethereum/hive/simulators/eth2/common/clients"
	"github.com/ethereum/hive/simulators/eth2/common/testnet"
	suite_base "github.com/ethereum/hive/simulators/eth2/dencun/suites/base"
)

type ReorgTestSpec struct {
	suite_base.BaseTestSpec

	// Array of weights for each chain, the last client re-orgs to the chain with the highest weight.
	// The length of the array also determines the number of chains to generate.
	ChainWeights []uint64
}

func (ts ReorgTestSpec) GetTestnetConfig(
	allNodeDefinitions clients.NodeDefinitions,
) *testnet.Config {
	// By default the last client does not validate and must sync to the other clients
	if ts.BaseTestSpec.ValidatingNodeCount == 0 {
		ts.BaseTestSpec.ValidatingNodeCount = ts.BaseTestSpec.NodeCount - 1
	}

	chains := ts.ChainWeights
	if len(chains) == 0 {
		chains = []uint64{1, 1}
	}

	if ts.NodeCount < (len(chains) + 1) {
		panic(fmt.Errorf("need at least %d nodes to generate %d different chains", len(chains)+1, len(chains)))
	}

	tc := ts.BaseTestSpec.GetTestnetConfig(allNodeDefinitions)

	for i := 0; i < len(tc.NodeDefinitions); i++ {
		if i == len(tc.NodeDefinitions)-1 {
			// The last node is the one that re-orgs all other nodes
			tc.NodeDefinitions[i].DisableStartup = true
		} else {
			// Other clients are disconnected from each other to form different chains
			tc.NodeDefinitions[i].ConsensusSubnet = fmt.Sprintf("%d", i%len(chains))
			tc.NodeDefinitions[i].ExecutionSubnet = fmt.Sprintf("%d", i%len(chains))
			tc.NodeDefinitions[i].ValidatorShares = chains[i%len(chains)]
		}
	}

	return tc
}
