package optimism

import (
	"fmt"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/hive/hivesim"
	"math/big"
)

func Eth1GenesisToParams(conf *params.ChainConfig) hivesim.Params {
	params := hivesim.Params{}
	addIfSet := func(name string, v *big.Int) {
		if v != nil {
			params[name] = v.String()
		}
	}
	addIfSet("HIVE_NETWORK_ID", conf.ChainID)
	addIfSet("HIVE_CHAIN_ID", conf.ChainID)
	addIfSet("HIVE_FORK_HOMESTEAD", conf.HomesteadBlock)
	addIfSet("HIVE_FORK_DAO_BLOCK", conf.DAOForkBlock)
	addIfSet("HIVE_FORK_TANGERINE", conf.EIP150Block)
	addIfSet("HIVE_FORK_SPURIOUS", conf.EIP155Block)
	addIfSet("HIVE_FORK_BYZANTIUM", conf.ByzantiumBlock)
	addIfSet("HIVE_FORK_CONSTANTINOPLE", conf.ConstantinopleBlock)
	addIfSet("HIVE_FORK_PETERSBURG", conf.PetersburgBlock)
	addIfSet("HIVE_FORK_ISTANBUL", conf.IstanbulBlock)
	addIfSet("HIVE_FORK_MUIRGLACIER", conf.MuirGlacierBlock)
	addIfSet("HIVE_FORK_BERLIN", conf.BerlinBlock)
	addIfSet("HIVE_FORK_LONDON", conf.LondonBlock)
	addIfSet("HIVE_FORK_ARROWGLACIER", conf.ArrowGlacierBlock)
	addIfSet("HIVE_MERGE_BLOCK_ID", conf.MergeNetsplitBlock)
	addIfSet("HIVE_TERMINAL_TOTAL_DIFFICULTY", conf.TerminalTotalDifficulty)
	if conf.Clique != nil {
		params["HIVE_CLIQUE_PERIOD"] = fmt.Sprint(conf.Clique.Period)
	}
	return params
}
