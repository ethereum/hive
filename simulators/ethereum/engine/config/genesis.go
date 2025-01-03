package config

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/hive/simulators/ethereum/engine/config/cancun"
)

func (f *ForkConfig) ConfigGenesis(genesis *core.Genesis) error {
	genesis.Config.MergeNetsplitBlock = big.NewInt(0)
	if f.ShanghaiTimestamp != nil {
		shanghaiTime := f.ShanghaiTimestamp.Uint64()
		genesis.Config.ShanghaiTime = &shanghaiTime
	}
	if f.CancunTimestamp != nil {
		if err := cancun.ConfigGenesis(genesis, f.CancunTimestamp.Uint64()); err != nil {
			return fmt.Errorf("failed to configure cancun fork: %v", err)
		}
	}
	return nil
}
