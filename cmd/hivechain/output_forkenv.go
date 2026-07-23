package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

// writeForkEnv writes chain fork configuration in the form that hive expects.
func (g *generator) writeForkEnv() error {
	cfg := g.genesis.Config
	env := make(map[string]string)

	// basic settings
	env["HIVE_CHAIN_ID"] = fmt.Sprint(cfg.ChainID)
	env["HIVE_NETWORK_ID"] = fmt.Sprint(cfg.ChainID)

	// system contracts
	env["HIVE_DEPOSIT_CONTRACT_ADDRESS"] = cfg.DepositContractAddress.Hex()

	// config consensus algorithm
	if cfg.Clique != nil {
		env["HIVE_CLIQUE_PERIOD"] = fmt.Sprint(cfg.Clique.Period)
	}

	// forks
	setNum := func(hive string, blocknum *big.Int) {
		if blocknum != nil {
			env[hive] = blocknum.Text(10)
		}
	}
	setTime := func(hive string, timestamp *uint64) {
		if timestamp != nil {
			env[hive] = fmt.Sprintf("%d", *timestamp)
		}
	}
	setNum("HIVE_FORK_HOMESTEAD", cfg.HomesteadBlock)
	setNum("HIVE_FORK_TANGERINE", cfg.EIP150Block)
	setNum("HIVE_FORK_SPURIOUS", cfg.EIP155Block)
	setNum("HIVE_FORK_BYZANTIUM", cfg.ByzantiumBlock)
	setNum("HIVE_FORK_CONSTANTINOPLE", cfg.ConstantinopleBlock)
	setNum("HIVE_FORK_PETERSBURG", cfg.PetersburgBlock)
	setNum("HIVE_FORK_ISTANBUL", cfg.IstanbulBlock)
	setNum("HIVE_FORK_MUIR_GLACIER", cfg.MuirGlacierBlock)
	setNum("HIVE_FORK_ARROW_GLACIER", cfg.ArrowGlacierBlock)
	setNum("HIVE_FORK_GRAY_GLACIER", cfg.GrayGlacierBlock)
	setNum("HIVE_FORK_BERLIN", cfg.BerlinBlock)
	setNum("HIVE_FORK_LONDON", cfg.LondonBlock)
	setNum("HIVE_MERGE_BLOCK_ID", cfg.MergeNetsplitBlock)
	setNum("HIVE_TERMINAL_TOTAL_DIFFICULTY", cfg.TerminalTotalDifficulty)
	setTime("HIVE_SHANGHAI_TIMESTAMP", cfg.ShanghaiTime)
	setTime("HIVE_CANCUN_TIMESTAMP", cfg.CancunTime)
	setTime("HIVE_PRAGUE_TIMESTAMP", cfg.PragueTime)
	setTime("HIVE_OSAKA_TIMESTAMP", cfg.OsakaTime)
	setTime("HIVE_BPO1_TIMESTAMP", cfg.BPO1Time)
	setTime("HIVE_BPO2_TIMESTAMP", cfg.BPO2Time)
	setTime("HIVE_AMSTERDAM_TIMESTAMP", cfg.AmsterdamTime)

	// blob schedule
	setBlobConfig := func(fork string, bc *params.BlobConfig) {
		if bc != nil {
			env["HIVE_"+fork+"_BLOB_TARGET"] = fmt.Sprint(bc.Target)
			env["HIVE_"+fork+"_BLOB_MAX"] = fmt.Sprint(bc.Max)
			env["HIVE_"+fork+"_BLOB_BASE_FEE_UPDATE_FRACTION"] = fmt.Sprint(bc.UpdateFraction)
		}
	}
	if cfg.BlobScheduleConfig != nil {
		setBlobConfig("CANCUN", cfg.BlobScheduleConfig.Cancun)
		setBlobConfig("PRAGUE", cfg.BlobScheduleConfig.Prague)
		setBlobConfig("OSAKA", cfg.BlobScheduleConfig.Osaka)
		setBlobConfig("BPO1", cfg.BlobScheduleConfig.BPO1)
		setBlobConfig("BPO2", cfg.BlobScheduleConfig.BPO2)
	}

	return g.writeJSON("forkenv.json", env)
}
