package suite_engine

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/hive/simulators/ethereum/engine/clmock"
	"github.com/ethereum/hive/simulators/ethereum/engine/config"
	"github.com/ethereum/hive/simulators/ethereum/engine/test"
)

// Runs a sanity test on a post Merge fork where a previous fork's (London) number is not zero
type NonZeroPreMergeFork struct {
	test.BaseSpec
}

func (s NonZeroPreMergeFork) WithMainFork(fork config.Fork) test.Spec {
	specCopy := s
	specCopy.MainFork = fork
	return specCopy
}

func (b NonZeroPreMergeFork) GetName() string {
	return "Pre-Merge Fork Number > 0"
}

func (s NonZeroPreMergeFork) GetForkConfig() *config.ForkConfig {
	forkConfig := s.BaseSpec.GetForkConfig()
	if forkConfig == nil {
		return nil
	}
	forkConfig.LondonNumber = common.Big1
	// All post merge forks must happen at the same time as the latest fork
	mainFork := s.GetMainFork()
	if mainFork == config.Cancun {
		forkConfig.ShanghaiTimestamp = new(big.Int).Set(forkConfig.CancunTimestamp)
	}
	return forkConfig
}

func (b NonZeroPreMergeFork) Execute(t *test.Env) {
	// Wait until TTD is reached by this client
	t.CLMock.WaitForTTD()

	// Simply produce a couple of blocks without transactions (if London is not active at genesis
	// we can't send type-2 transactions) and check that the chain progresses without issues
	t.CLMock.ProduceBlocks(5, clmock.BlockProcessCallbacks{})
}
