package optimism

import (
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
)

func Eth1GenesisToParams(conf *params.ChainConfig) hivesim.Params {
	params := hivesim.Params{
		"HIVE_NETWORK_ID":     conf.ChainID.String(),
		"HIVE_CHAIN_ID":       conf.ChainID.String(),
		"HIVE_FORK_HOMESTEAD": conf.HomesteadBlock.String(),
		//"HIVE_FORK_DAO_BLOCK":           conf.DAOForkBlock.String(),  // nil error, not used anyway
		"HIVE_FORK_TANGERINE":            conf.EIP150Block.String(),
		"HIVE_FORK_SPURIOUS":             conf.EIP155Block.String(), // also eip558
		"HIVE_FORK_BYZANTIUM":            conf.ByzantiumBlock.String(),
		"HIVE_FORK_CONSTANTINOPLE":       conf.ConstantinopleBlock.String(),
		"HIVE_FORK_PETERSBURG":           conf.PetersburgBlock.String(),
		"HIVE_FORK_ISTANBUL":             conf.IstanbulBlock.String(),
		"HIVE_FORK_MUIRGLACIER":          conf.MuirGlacierBlock.String(),
		"HIVE_FORK_BERLIN":               conf.BerlinBlock.String(),
		"HIVE_FORK_LONDON":               conf.LondonBlock.String(),
		"HIVE_FORK_ARROWGLACIER":         conf.ArrowGlacierBlock.String(),
		"HIVE_MERGE_BLOCK_ID":            conf.MergeNetsplitBlock.String(),
		"HIVE_TERMINAL_TOTAL_DIFFICULTY": conf.TerminalTotalDifficulty.String(),
	}
	if conf.Clique != nil {
		params["HIVE_CLIQUE_PERIOD"] = fmt.Sprint(conf.Clique.Period)
	}
	return params
}
