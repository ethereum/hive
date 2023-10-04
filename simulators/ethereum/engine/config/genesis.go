package config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
)

func (f *ForkConfig) ConfigGenesis(genesis *core.Genesis) error {
	if f.ShanghaiTimestamp != nil {
		shanghaiTime := f.ShanghaiTimestamp.Uint64()
		genesis.Config.ShanghaiTime = &shanghaiTime

		if genesis.Timestamp >= shanghaiTime {
			// Remove PoW altogether
			genesis.Difficulty = common.Big0
			genesis.Config.TerminalTotalDifficulty = common.Big0
			genesis.Config.Clique = nil
			genesis.ExtraData = []byte{}
		}
	}
	if f.CancunTimestamp != nil {
		if err := cancun.ConfigGenesis(genesis, f.CancunTimestamp.Uint64()); err != nil {
			return fmt.Errorf("failed to configure cancun fork: %v", err)
		}
	}
	return nil
}
